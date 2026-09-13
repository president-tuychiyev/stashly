// Package jobs contains the queued background work of the service.
package jobs

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goravel/framework/support/carbon"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
	"github.com/president-tuychiyev/stashly/api/app/support/safeurl"
)

// zipMaxAttempts is how many times the worker calls Handle for one archive
// before the archive is parked in the failed state. Goravel retries in
// process: Worker.call() asks ShouldRetry(err, attempt) and calls Handle
// again, so this is the whole retry budget for one dispatch.
const zipMaxAttempts = 3

// zipRetryDelay is how long the worker waits between two attempts.
const zipRetryDelay = 5 * time.Second

// ZipFolders streams the requested folders of a client into a single zip file.
type ZipFolders struct{}

// Signature is the unique name the queue uses to find this job.
func (r *ZipFolders) Signature() string {
	return services.ZipFoldersSignature
}

// archiveFailure carries the archive id along with the error, so ShouldRetry
// knows which row to park once the retries are used up.
type archiveFailure struct {
	archiveID uint
	err       error
}

func (r *archiveFailure) Error() string {
	return r.err.Error()
}

func (r *archiveFailure) Unwrap() error {
	return r.err
}

// ShouldRetry implements queue.JobWithShouldRetry. It is the only place that
// knows whether an attempt was the last one, so it is also the place that
// turns a temporarily failed archive into a permanently failed one: Handle
// leaves the row in "pending" so the next attempt can claim it again, and only
// the final attempt writes "failed" and fires the callback.
func (r *ZipFolders) ShouldRetry(err error, attempt int) (bool, time.Duration) {
	if attempt < zipMaxAttempts {
		return true, zipRetryDelay
	}

	var failure *archiveFailure
	if errors.As(err, &failure) {
		r.markFailed(failure.archiveID, failure.err)
	}

	return false, 0
}

// markFailed records the final outcome of an archive and notifies the caller.
func (r *ZipFolders) markFailed(archiveID uint, cause error) {
	var archive models.Archive
	if err := facades.Orm().Query().Where("id", archiveID).First(&archive); err != nil || archive.ID == 0 {
		facades.Log().Error(fmt.Sprintf("could not load archive %d to mark it failed", archiveID))

		return
	}

	message := cause.Error()
	archive.Status = models.ArchiveStatusFailed
	archive.Error = &message
	archive.FinishedAt = carbon.NewDateTime(carbon.Now())
	if err := facades.Orm().Query().Save(&archive); err != nil {
		facades.Log().Error("failed to mark archive as failed: " + err.Error())
	}

	r.callback(&archive)
}

// Handle expects a single argument, the id of the archive to build.
func (r *ZipFolders) Handle(args ...any) error {
	if len(args) == 0 {
		return errors.New("zip_folders requires an archive id")
	}

	archiveID, ok := args[0].(uint)
	if !ok {
		return fmt.Errorf("zip_folders got an unexpected archive id %v", args[0])
	}

	var archive models.Archive
	if err := facades.Orm().Query().Where("id", archiveID).First(&archive); err != nil {
		return err
	}
	if archive.ID == 0 {
		return fmt.Errorf("archive %d not found", archiveID)
	}
	if archive.Status == models.ArchiveStatusDone || archive.Status == models.ArchiveStatusExpired {
		return nil
	}

	// Claim the archive with a conditional update so that two workers picking
	// the same message up cannot both start building it. "failed" is claimable
	// as well: a re-dispatch of an archive that gave up earlier, either by hand
	// or by the recovery pass, has to be able to take it back.
	claimed, err := facades.Orm().Query().Model(&models.Archive{}).
		Where("id", archive.ID).
		WhereIn("status", []any{models.ArchiveStatusPending, models.ArchiveStatusFailed}).
		Update(map[string]any{
			"status":   models.ArchiveStatusProcessing,
			"progress": 0,
			"error":    nil,
		})
	if err != nil {
		return err
	}
	if claimed.RowsAffected == 0 {
		// Nothing was updated, so the row moved out of a claimable state
		// between the read above and the update. Say which of the two it was:
		// somebody else is building it right now, or it is in a state this job
		// must not touch at all.
		var current models.Archive
		if readErr := facades.Orm().Query().Where("id", archive.ID).First(&current); readErr == nil && current.ID != 0 {
			if current.Status == models.ArchiveStatusProcessing {
				facades.Log().Warning(fmt.Sprintf("archive %d was claimed elsewhere, skipping", archive.ID))
			} else {
				facades.Log().Warning(fmt.Sprintf("archive %d is not claimable (status %s), skipping", archive.ID, current.Status))
			}
		} else {
			facades.Log().Warning(fmt.Sprintf("archive %d is not claimable, skipping", archive.ID))
		}

		return nil
	}
	archive.Status = models.ArchiveStatusProcessing
	archive.Progress = 0
	archive.Error = nil

	if err := r.build(&archive); err != nil {
		// Hand the row back to the "pending" state so the retry can claim it
		// again, and keep the error visible in the meantime. Only ShouldRetry,
		// which knows the attempt number, writes the terminal "failed" state.
		message := err.Error()
		archive.Status = models.ArchiveStatusPending
		archive.Progress = 0
		archive.Error = &message
		if saveErr := facades.Orm().Query().Save(&archive); saveErr != nil {
			facades.Log().Error("failed to reset a failing archive to pending: " + saveErr.Error())
		}

		return &archiveFailure{archiveID: archive.ID, err: err}
	}

	r.callback(&archive)

	return nil
}

func (r *ZipFolders) build(archive *models.Archive) error {
	storage := services.NewStorageService()

	files, err := r.collect(archive)
	if err != nil {
		return err
	}

	archive.FilesCount = len(files)
	if err := facades.Orm().Query().Save(archive); err != nil {
		return err
	}

	relative := fmt.Sprintf("%d-%s.zip", archive.ID, uuid.NewString())
	target, err := storage.ResolvePath("archives", relative)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	output, err := os.Create(target)
	if err != nil {
		return err
	}

	writer := zip.NewWriter(output)
	used := map[string]int{}
	lastReported := 0

	for index := range files {
		file := files[index]

		source, openErr := storage.AbsolutePath(&file)
		if openErr != nil {
			continue
		}

		handle, openErr := os.Open(source)
		if openErr != nil {
			// A record without a file on disk should not fail the whole archive.
			facades.Log().Warning("archive skipped a missing file: " + file.Path)
			continue
		}

		entryName := r.entryName(&file, used)
		header := &zip.FileHeader{Name: entryName, Method: zip.Deflate}
		if info, statErr := handle.Stat(); statErr == nil {
			header.Modified = info.ModTime()
		}
		entry, createErr := writer.CreateHeader(header)
		if createErr != nil {
			handle.Close()
			writer.Close()
			output.Close()
			_ = os.Remove(target)

			return createErr
		}

		if _, copyErr := io.Copy(entry, handle); copyErr != nil {
			handle.Close()
			writer.Close()
			output.Close()
			_ = os.Remove(target)

			return copyErr
		}
		handle.Close()

		// Report progress every 2 percent or every 50 files, whichever is first.
		progress := (index + 1) * 100 / max(len(files), 1)
		if progress-lastReported >= 2 || (index+1)%50 == 0 {
			lastReported = progress
			archive.Progress = progress
			if err := facades.Orm().Query().Save(archive); err != nil {
				facades.Log().Warning("failed to update archive progress: " + err.Error())
			}
		}
	}

	if err := writer.Close(); err != nil {
		output.Close()
		_ = os.Remove(target)

		return err
	}
	if err := output.Close(); err != nil {
		_ = os.Remove(target)

		return err
	}

	info, err := os.Stat(target)
	if err != nil {
		_ = os.Remove(target)

		return err
	}

	size := info.Size()
	storedPath := "archives/" + relative
	ttl := facades.Config().GetInt("storage.archive_ttl_days")
	if ttl <= 0 {
		ttl = 5
	}

	archive.Status = models.ArchiveStatusDone
	archive.Progress = 100
	archive.Size = &size
	archive.Path = &storedPath
	archive.Error = nil
	// The contract dates the retention window from the moment the archive was
	// requested, not from the moment the worker happened to finish it.
	// carbon.AddDays mutates the receiver, so work on a copy of CreatedAt.
	createdAt := carbon.Now()
	if archive.CreatedAt != nil && archive.CreatedAt.Carbon != nil && !archive.CreatedAt.IsZero() {
		createdAt = archive.CreatedAt.Copy()
	}
	archive.ExpiresAt = carbon.NewDateTime(createdAt.AddDays(ttl))
	archive.FinishedAt = carbon.NewDateTime(carbon.Now())

	return facades.Orm().Query().Save(archive)
}

// collect gathers the files that belong to the requested folders.
func (r *ZipFolders) collect(archive *models.Archive) ([]models.File, error) {
	query := facades.Orm().Query().Model(&models.File{}).Where("client_id", archive.ClientID)

	folders := archive.Folders
	everything := len(folders) == 0
	for _, folder := range folders {
		if folder == "*" || folder == "" {
			everything = true
		}
	}

	if !everything {
		conditions := make([]string, 0, len(folders))
		values := make([]any, 0, len(folders)*2)
		for _, folder := range folders {
			conditions = append(conditions, "(folder = ? OR folder LIKE ?)")
			values = append(values, folder, folder+"/%")
		}
		query = query.Where("("+strings.Join(conditions, " OR ")+")", values...)
	}

	var files []models.File
	if err := query.OrderBy("id").Get(&files); err != nil {
		return nil, err
	}

	return files, nil
}

// entryName builds a unique, safe path for a file inside the zip.
func (r *ZipFolders) entryName(file *models.File, used map[string]int) string {
	name := services.SanitizeFileName(file.OriginalName)
	entry := name
	if file.Folder != "" {
		entry = path.Join(file.Folder, name)
	}

	if count, exists := used[entry]; exists {
		extension := path.Ext(name)
		base := strings.TrimSuffix(name, extension)

		// Keep counting until a name nobody has taken yet turns up: the
		// generated name may itself collide with a real file in the folder.
		for {
			unique := fmt.Sprintf("%s (%d)%s", base, count, extension)
			if file.Folder != "" {
				unique = path.Join(file.Folder, unique)
			}
			count++

			if _, taken := used[unique]; !taken {
				used[entry] = count
				used[unique] = 1

				return unique
			}
		}
	}

	used[entry] = 1

	return entry
}

// callback notifies the caller supplied webhook. Failures are logged only.
func (r *ZipFolders) callback(archive *models.Archive) {
	if archive.CallbackURL == nil || *archive.CallbackURL == "" {
		return
	}

	body, err := json.Marshal(map[string]any{"data": resources.Archive(archive, "/api/archives")})
	if err != nil {
		facades.Log().Warning("failed to encode archive callback payload: " + err.Error())
		return
	}

	// The URL was validated when the archive was created, but DNS may have
	// changed since; the safe client re-checks the resolved IP at dial time
	// and re-validates every redirect.
	if err := safeurl.Validate(*archive.CallbackURL); err != nil {
		facades.Log().Warning("refusing to call an unsafe archive callback url: " + err.Error())

		return
	}

	client := safeurl.Client(10 * time.Second)

	for attempt := 1; attempt <= 3; attempt++ {
		if r.postCallback(client, *archive.CallbackURL, body, attempt) {
			return
		}
	}
}

// postCallback performs one callback attempt and reports whether it succeeded.
// It never panics, a broken webhook must not take the worker down.
func (r *ZipFolders) postCallback(client *http.Client, url string, body []byte, attempt int) (delivered bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			facades.Log().Warning(fmt.Sprintf("archive callback panicked: %v", recovered))
			delivered = false
		}
	}()

	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		facades.Log().Warning("invalid archive callback url: " + err.Error())

		return true
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		facades.Log().Warning(fmt.Sprintf("archive callback attempt %d failed: %s", attempt, err.Error()))

		return false
	}
	defer response.Body.Close()
	// Drain enough of the body to let the connection be reused, but never let a
	// hostile webhook stream gigabytes into this worker.
	_, _ = io.CopyN(io.Discard, response.Body, 64<<10)

	if response.StatusCode < 400 {
		return true
	}

	facades.Log().Warning(fmt.Sprintf("archive callback attempt %d returned %d", attempt, response.StatusCode))

	return false
}
