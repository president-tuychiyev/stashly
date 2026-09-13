// Package commands holds the Artisan commands of this service.
package commands

import (
	"strconv"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
	"github.com/goravel/framework/support/carbon"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type ArchivesCleanup struct{}

func NewArchivesCleanup() *ArchivesCleanup {
	return &ArchivesCleanup{}
}

func (r *ArchivesCleanup) Signature() string {
	return "archives:cleanup"
}

func (r *ArchivesCleanup) Description() string {
	return "Delete the zip files of expired archives and mark those archives as expired"
}

func (r *ArchivesCleanup) Extend() command.Extend {
	return command.Extend{Category: "archives"}
}

func (r *ArchivesCleanup) Handle(ctx console.Context) error {
	archives := services.NewArchiveService()

	var expired []models.Archive
	err := facades.Orm().Query().Model(&models.Archive{}).
		Where("status", models.ArchiveStatusDone).
		WhereNotNull("expires_at").
		Where("expires_at < ?", carbon.Now().StdTime()).
		Get(&expired)
	if err != nil {
		return err
	}

	for index := range expired {
		archive := expired[index]
		archives.RemoveFile(&archive)

		archive.Status = models.ArchiveStatusExpired
		archive.Path = nil
		archive.Size = nil
		if err := facades.Orm().Query().Save(&archive); err != nil {
			ctx.Error("failed to expire archive: " + err.Error())
		}
	}

	services.NewAuditService().Log(services.SystemActor(), "archives:cleanup", "archive", nil, map[string]any{
		"expired": len(expired),
	}, "")

	ctx.Info("expired archives: " + strconv.Itoa(len(expired)))

	return nil
}
