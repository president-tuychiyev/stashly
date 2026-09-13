package services

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

// SyncReport summarises one reconciliation run between database and disk.
type SyncReport struct {
	OrphansOnDisk  int `json:"orphans_on_disk"`
	MissingOnDisk  int `json:"missing_on_disk"`
	RemovedRecords int `json:"removed_records"`
	RemovedFiles   int `json:"removed_files"`
	// Errors counts the records that could not be judged at all, because the
	// path could not be resolved or stat() failed with something other than
	// "does not exist". Those rows are left alone on purpose: a permission
	// error or an unreachable mount is not proof that the file is gone.
	Errors int `json:"errors"`
}

// Sync compares the files table with the public and private disks. Records
// without a file are deleted, files without a record are removed from disk.
func (r *StorageService) Sync() (SyncReport, error) {
	report := SyncReport{}

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

// visibilityOf maps a disk name back onto the visibility column.
func visibilityOf(disk string) string {
	if disk == "private" {
		return models.VisibilityPrivate
	}

	return models.VisibilityPublic
}
