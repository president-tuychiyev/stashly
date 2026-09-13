package bootstrap

import (
	"os"
	"time"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// RecoverArchives puts archives that were being built when the process died
// back into the queue. It runs once, when the HTTP server boots.
func RecoverArchives() {
	// Artisan commands should not touch the queue, they may even be the
	// migration that creates the tables this function reads.
	if len(os.Args) > 1 && os.Args[1] == "artisan" {
		return
	}

	archives := services.NewArchiveService()

	// Archives that were mid build when the process died.
	var stuck []models.Archive
	if err := facades.Orm().Query().Model(&models.Archive{}).
		Where("status", models.ArchiveStatusProcessing).Get(&stuck); err != nil {
		facades.Log().Warning("could not look for interrupted archives: " + err.Error())
	}

	for index := range stuck {
		archive := stuck[index]
		archive.Status = models.ArchiveStatusPending
		archive.Progress = 0
		if err := facades.Orm().Query().Save(&archive); err != nil {
			facades.Log().Warning("could not reset an interrupted archive: " + err.Error())
			continue
		}

		if err := archives.Dispatch(archive.ID); err != nil {
			facades.Log().Warning("could not re-queue an interrupted archive: " + err.Error())
		}
	}

	// Archives whose queue message was lost: still pending well after they
	// were created, so nothing is going to pick them up on its own. One minute
	// is long enough that a job just being handed to a worker is not touched.
	var orphaned []models.Archive
	if err := facades.Orm().Query().Model(&models.Archive{}).
		Where("status", models.ArchiveStatusPending).
		Where("created_at < ?", time.Now().Add(-time.Minute)).Get(&orphaned); err != nil {
		facades.Log().Warning("could not look for undispatched archives: " + err.Error())

		return
	}

	for index := range orphaned {
		if err := archives.Dispatch(orphaned[index].ID); err != nil {
			facades.Log().Warning("could not re-queue an undispatched archive: " + err.Error())
		}
	}
}
