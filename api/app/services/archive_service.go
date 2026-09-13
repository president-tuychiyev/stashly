package services

import (
	"os"
	"strings"

	"github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/support/carbon"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

// ZipFoldersSignature is the queue signature of the job that builds the zip.
const ZipFoldersSignature = "zip_folders"

type ArchiveService struct {
	storage *StorageService
}

func NewArchiveService() *ArchiveService {
	return &ArchiveService{storage: NewStorageService()}
}

// NormalizeFolders validates the requested folders. An empty list or ["*"]
// means the whole client root.
func (r *ArchiveService) NormalizeFolders(folders []string) ([]string, error) {
	if len(folders) == 0 {
		return []string{"*"}, nil
	}

	normalized := make([]string, 0, len(folders))
	for _, folder := range folders {
		folder = strings.TrimSpace(folder)
		if folder == "*" {
			return []string{"*"}, nil
		}

		clean, err := r.storage.NormalizeFolder(folder)
		if err != nil {
			return nil, err
		}
		if clean == "" {
			return []string{"*"}, nil
		}
		normalized = append(normalized, clean)
	}

	return normalized, nil
}

// Create records a new archive request and queues the job that builds it.
func (r *ArchiveService) Create(clientID uint, folders []string, callbackURL *string, createdByType string, createdByID uint) (*models.Archive, error) {
	archive := &models.Archive{
		ClientID:      clientID,
		CreatedByType: createdByType,
		CreatedByID:   createdByID,
		Folders:       folders,
		Status:        models.ArchiveStatusPending,
		CallbackURL:   callbackURL,
	}

	if err := facades.Orm().Query().Create(archive); err != nil {
		return nil, err
	}

	// A row that was never queued would sit in "pending" for ever, so a failed
	// dispatch is recorded on the archive itself and reported to the caller.
	if err := r.Dispatch(archive.ID); err != nil {
		message := "could not queue the archive job: " + err.Error()
		archive.Status = models.ArchiveStatusFailed
		archive.Error = &message
		archive.FinishedAt = carbon.NewDateTime(carbon.Now())
		if saveErr := facades.Orm().Query().Save(archive); saveErr != nil {
			facades.Log().Error("failed to mark an undispatched archive as failed: " + saveErr.Error())
		}

		return archive, err
	}

	return archive, nil
}

// Dispatch queues the zip job for an existing archive.
func (r *ArchiveService) Dispatch(archiveID uint) error {
	job, err := facades.Queue().GetJob(ZipFoldersSignature)
	if err != nil {
		return err
	}

	return facades.Queue().Job(job, []queue.Arg{{Type: "uint", Value: archiveID}}).Dispatch()
}

// Delete removes an archive together with its zip file.
func (r *ArchiveService) Delete(archive *models.Archive) error {
	r.RemoveFile(archive)

	_, err := facades.Orm().Query().Delete(archive)

	return err
}

// RemoveFile deletes the generated zip from disk, if there is one.
func (r *ArchiveService) RemoveFile(archive *models.Archive) {
	if archive.Path == nil || *archive.Path == "" {
		return
	}

	target, err := r.storage.ResolvePath("archives", strings.TrimPrefix(*archive.Path, "archives/"))
	if err != nil {
		return
	}

	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		facades.Log().Warning("failed to remove archive file: " + err.Error())
	}
}
