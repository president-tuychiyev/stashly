package commands

import (
	"fmt"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"

	"github.com/president-tuychiyev/stashly/api/app/services"
)

type StorageSync struct{}

func NewStorageSync() *StorageSync {
	return &StorageSync{}
}

func (r *StorageSync) Signature() string {
	return "storage:sync"
}

func (r *StorageSync) Description() string {
	return "Reconcile the files table with what is actually stored on disk"
}

func (r *StorageSync) Extend() command.Extend {
	return command.Extend{
		Category: "storage",
		Flags: []command.Flag{
			&command.StringFlag{
				Name:  "orphans",
				Usage: "what to do with files on disk without a record: delete or register",
				Value: services.SyncOrphansDelete,
			},
		},
	}
}

func (r *StorageSync) Handle(ctx console.Context) error {
	orphans := ctx.Option("orphans")
	if orphans == "" {
		orphans = services.SyncOrphansDelete
	}

	report, err := services.NewStorageService().Sync(orphans)
	if err != nil {
		return err
	}

	services.NewAuditService().Log(services.SystemActor(), "storage:sync", "", nil, map[string]any{
		"orphans_on_disk":  report.OrphansOnDisk,
		"missing_on_disk":  report.MissingOnDisk,
		"removed_records":  report.RemovedRecords,
		"removed_files":    report.RemovedFiles,
		"registered_files": report.RegisteredFiles,
		"skipped_files":    report.SkippedFiles,
		"errors":           report.Errors,
		"orphans":          orphans,
	}, "")

	ctx.Info(fmt.Sprintf(
		"orphans=%s orphans_on_disk=%d missing_on_disk=%d removed_records=%d removed_files=%d registered_files=%d skipped_files=%d errors=%d",
		orphans, report.OrphansOnDisk, report.MissingOnDisk, report.RemovedRecords, report.RemovedFiles,
		report.RegisteredFiles, report.SkippedFiles, report.Errors,
	))

	return nil
}
