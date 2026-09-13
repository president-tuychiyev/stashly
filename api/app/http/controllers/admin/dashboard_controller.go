package admin

import (
	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type DashboardController struct{}

func NewDashboardController() *DashboardController {
	return &DashboardController{}
}

type clientUsageRow struct {
	ClientID   uint   `gorm:"column:client_id" json:"client_id"`
	Name       string `gorm:"column:name" json:"name"`
	FilesCount int64  `gorm:"column:files_count" json:"files_count"`
	Size       int64  `gorm:"column:size" json:"size"`
}

type uploadDayRow struct {
	Date  string `gorm:"column:date" json:"date"`
	Count int64  `gorm:"column:count" json:"count"`
	Size  int64  `gorm:"column:size" json:"size"`
}

// Index returns the numbers shown on the admin landing page. Every figure is
// narrowed to the clients the caller owns, so two admins never see each
// other's totals.
func (r *DashboardController) Index(ctx http.Context) http.Response {
	scope := services.Scope(ctx)
	files := func() orm.Query {
		return scope.ApplyToClientColumn(facades.Orm().Query().Model(&models.File{}), "client_id")
	}

	clientsCount, err := scope.ApplyToClients(facades.Orm().Query().Model(&models.Client{})).Count()
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	filesCount, err := files().Count()
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	var totalSize *int64
	if err := files().Sum("size", &totalSize); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}
	if totalSize == nil {
		zero := int64(0)
		totalSize = &zero
	}

	archivesPending, err := scope.ApplyToClientColumn(
		facades.Orm().Query().Model(&models.Archive{}), "client_id").
		WhereIn("status", []any{models.ArchiveStatusPending, models.ArchiveStatusProcessing}).Count()
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	byClient := []clientUsageRow{}
	err = files().
		Join("left join clients on clients.id = files.client_id").
		Select("files.client_id as client_id, clients.name as name, count(*) as files_count, coalesce(sum(files.size), 0) as size").
		GroupBy("files.client_id", "clients.name").
		OrderByRaw("coalesce(sum(files.size), 0) desc").
		Get(&byClient)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	uploads := []uploadDayRow{}
	err = files().
		Where("created_at >= now() - interval '30 days'").
		Select("to_char(created_at at time zone 'UTC', 'YYYY-MM-DD') as date, count(*) as count, coalesce(sum(size), 0) as size").
		GroupBy("date").
		OrderBy("date").
		Get(&uploads)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return responses.Data(ctx, http.StatusOK, map[string]any{
		"clients_count":        clientsCount,
		"files_count":          filesCount,
		"total_size":           *totalSize,
		"archives_pending":     archivesPending,
		"by_client":            byClient,
		"uploads_last_30_days": uploads,
	})
}
