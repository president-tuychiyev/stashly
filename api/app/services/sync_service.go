package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/goravel/framework/support/carbon"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

const (
	SyncOrphansDelete   = "delete"
	SyncOrphansRegister = "register"
)

var ErrInvalidOrphansMode = errors.New("orphans must be delete or register")

// SyncReport summarises one reconciliation run between database and disk.
type SyncReport struct {
	OrphansOnDisk   int `json:"orphans_on_disk"`
	MissingOnDisk   int `json:"missing_on_disk"`
	RemovedRecords  int `json:"removed_records"`
	RemovedFiles    int `json:"removed_files"`
	RegisteredFiles int `json:"registered_files"`
	SkippedFiles    int `json:"skipped_files"`
	// Errors counts the records that could not be judged at all, because the
	// path could not be resolved or stat() failed with something other than
	// "does not exist". Those rows are left alone on purpose: a permission
	// error or an unreachable mount is not proof that the file is gone.
	Errors int `json:"errors"`
}

// Sync compares the files table with the public and private disks. Records
// without a file are deleted. Files without a record are removed from disk, or
// registered in place when orphans is SyncOrphansRegister.
func (r *StorageService) Sync(orphans string) (SyncReport, error) {
	report := SyncReport{}

	if orphans == "" {
		orphans = SyncOrphansDelete
	}
	if orphans != SyncOrphansDelete && orphans != SyncOrphansRegister {
		return report, ErrInvalidOrphansMode
	}
	clients := map[uint]bool{}

	// Everything written after this moment is younger than the snapshot below
	// and therefore cannot be judged against it. One minute of margin covers
	// clock skew between the database and the file system.
	snapshot := time.Now().Add(-time.Minute)

	var files []models.File
	if err := facades.Orm().Query().Model(&models.File{}).Get(&files); err != nil {
		return report, err
	}

	known := map[string]struct{}{}
	for index := range files {
		known[files[index].Disk()+":"+files[index].Path] = struct{}{}
	}

	// Records whose file disappeared from disk.
	for index := range files {
		target, err := r.AbsolutePath(&files[index])
		if err != nil {
			report.Errors++
			facades.Log().Warning("storage sync could not resolve a file path: " + files[index].Path + ": " + err.Error())

			continue
		}

		if _, err := os.Stat(target); err != nil {
			// Only a file that is provably absent may cost its row. Anything
			// else (EACCES, EIO, a mount that is not there right now) is
			// reported and skipped, never deleted.
			if !os.IsNotExist(err) {
				report.Errors++
				facades.Log().Warning("storage sync could not stat a file: " + target + ": " + err.Error())

				continue
			}

			report.MissingOnDisk++
			if _, err := facades.Orm().Query().Delete(&files[index]); err == nil {
				report.RemovedRecords++
			}
		}
	}

	// Files on disk nobody knows about.
	for _, disk := range []string{"public", "private"} {
		root := r.DiskRoot(disk)
		walkErr := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				return nil
			}

			relative, relErr := filepath.Rel(root, current)
			if relErr != nil {
				return nil
			}
			relative = filepath.ToSlash(relative)
			if strings.HasPrefix(relative, ".") {
				return nil
			}

			if _, ok := known[disk+":"+relative]; ok {
				return nil
			}

			// An upload that landed after the snapshot was taken is simply not
			// in it yet; deleting it would destroy a file that is perfectly
			// well recorded in the database.
			if info, statErr := entry.Info(); statErr == nil && info.ModTime().After(snapshot) {
				return nil
			}

			if orphans == SyncOrphansRegister {
				r.registerOrphan(disk, current, relative, clients, &report)

				return nil
			}

			// The snapshot may also be stale by now, so ask the database once
			// more right before the unlink.
			exists, existsErr := facades.Orm().Query().Model(&models.File{}).
				Where("path", relative).Where("visibility", visibilityOf(disk)).Exists()
			if existsErr != nil || exists {
				return nil
			}

			report.OrphansOnDisk++
			if removeErr := os.Remove(current); removeErr == nil {
				report.RemovedFiles++
			}

			return nil
		})
		if walkErr != nil {
			return report, walkErr
		}
	}

	return report, nil
}

func (r *StorageService) registerOrphan(disk, current, relative string, clients map[uint]bool, report *SyncReport) {
	info, err := os.Lstat(current)
	if err != nil || !info.Mode().IsRegular() {
		return
	}

	trashed, err := facades.Orm().Query().WithTrashed().Model(&models.File{}).Where("path", relative).Exists()
	if err != nil {
		return
	}
	report.OrphansOnDisk++
	if trashed {
		report.SkippedFiles++

		return
	}

	clientID, folder, ok := ownerAndFolder(relative)
	if ok {
		known, cached := clients[clientID]
		if !cached {
			known, err = facades.Orm().Query().Model(&models.Client{}).Where("id", clientID).Exists()
			if err != nil {
				report.Errors++

				return
			}
			clients[clientID] = known
		}
		ok = known
	}
	if !ok {
		report.SkippedFiles++

		return
	}

	record, err := buildFileRecord(disk, current, relative, clientID, folder, info)
	if err == nil {
		err = facades.Orm().Query().Create(record)
	}
	if err != nil {
		report.Errors++
		facades.Log().Warning("storage sync could not register a file: " + relative + ": " + err.Error())

		return
	}

	report.RegisteredFiles++
}

func ownerAndFolder(relative string) (uint, string, bool) {
	parts := strings.Split(relative, "/")
	if len(parts) < 2 {
		return 0, "", false
	}

	id, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != parts[0] {
		return 0, "", false
	}

	return uint(id), strings.Join(parts[1:len(parts)-1], "/"), true
}

func buildFileRecord(disk, current, relative string, clientID uint, folder string, info os.FileInfo) (*models.File, error) {
	source, err := os.Open(current)
	if err != nil {
		return nil, err
	}
	defer source.Close()

	head := make([]byte, 512)
	read, err := io.ReadFull(source, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}

	digest := sha256.New()
	digest.Write(head[:read])
	rest, err := io.Copy(digest, source)
	if err != nil {
		return nil, err
	}

	name := filepath.Base(current)
	extension := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	if len(extension) > 20 {
		extension = extension[:20]
	}

	detected := trimMimeParameters(http.DetectContentType(head[:read]))
	if detected == "application/octet-stream" && extension != "" {
		if guess := trimMimeParameters(mime.TypeByExtension("." + extension)); guess != "" {
			detected = guess
		}
	}

	modified := carbon.NewDateTime(carbon.FromStdTime(info.ModTime()))

	record := &models.File{
		ClientID:     clientID,
		Folder:       folder,
		Name:         name,
		OriginalName: name,
		Path:         relative,
		Mime:         detected,
		Extension:    extension,
		Size:         int64(read) + rest,
		Sha256:       hex.EncodeToString(digest.Sum(nil)),
		Visibility:   visibilityOf(disk),
	}
	record.CreatedAt = modified
	record.UpdatedAt = modified

	return record, nil
}

// visibilityOf maps a disk name back onto the visibility column.
func visibilityOf(disk string) string {
	if disk == "private" {
		return models.VisibilityPrivate
	}

	return models.VisibilityPublic
}
