package admin

import (
	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type StorageController struct {
	storage *services.StorageService
}

func NewStorageController() *StorageController {
	return &StorageController{storage: services.NewStorageService()}
}

// Sync reconciles the files table with what is actually on disk.
func (r *StorageController) Sync(ctx http.Context) http.Response {
	// The sync walks the whole disk and can delete rows of any client, so it
	// stays a super admin tool even though can_user lets the admin role
	// through every other /admin route.
	if !services.Scope(ctx).IsSuperAdmin {
		return responses.Error(ctx, http.StatusForbidden, "Forbidden")
	}

	report, err := r.storage.Sync()
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return ctx.Response().Success().Json(http.Json{
		"orphans_on_disk": report.OrphansOnDisk,
		"missing_on_disk": report.MissingOnDisk,
		"removed_records": report.RemovedRecords,
		"removed_files":   report.RemovedFiles,
		"errors":          report.Errors,
	})
}
