package admin

import (
	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type AuditLogController struct{}

func NewAuditLogController() *AuditLogController {
	return &AuditLogController{}
}

// Index lists the audit trail, newest first.
func (r *AuditLogController) Index(ctx http.Context) http.Response {
	pagination := responses.ReadPagination(ctx)

	query := services.Scope(ctx).ApplyToAuditLogs(facades.Orm().Query().Model(&models.AuditLog{}))
	if actorType := ctx.Request().Query("actor_type"); actorType != "" {
		query = query.Where("actor_type", actorType)
	}
	if action := ctx.Request().Query("action"); action != "" {
		query = query.Where("action", action)
	}
	if subjectType := ctx.Request().Query("subject_type"); subjectType != "" {
		query = query.Where("subject_type", subjectType)
	}

	var logs []models.AuditLog
	var total int64
	if err := query.OrderByDesc("id").Paginate(pagination.Page, pagination.PerPage, &logs, &total); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return responses.Paginated(ctx, resources.AuditLogs(logs), pagination, total)
}
