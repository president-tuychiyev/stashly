package admin

import (
	"strings"

	"github.com/goravel/framework/contracts/http"
	"github.com/spf13/cast"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	apicontrollers "github.com/president-tuychiyev/stashly/api/app/http/controllers/api"
	"github.com/president-tuychiyev/stashly/api/app/http/requests"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
	"github.com/president-tuychiyev/stashly/api/app/support/safeurl"
)

// AdminArchivePrefix is used to build the download url of an archive.
const AdminArchivePrefix = "/admin/archives"

type ArchiveController struct {
	archives *services.ArchiveService
	audit    *services.AuditService
}

func NewArchiveController() *ArchiveController {
	return &ArchiveController{archives: services.NewArchiveService(), audit: services.NewAuditService()}
}

// Index lists archives across clients.
func (r *ArchiveController) Index(ctx http.Context) http.Response {
	pagination := responses.ReadPagination(ctx)

	query := services.Scope(ctx).ApplyToClientColumn(
		facades.Orm().Query().Model(&models.Archive{}).With("Client"), "client_id")
	if clientID := ctx.Request().QueryInt("client_id", 0); clientID > 0 {
		query = query.Where("client_id", clientID)
	}
	if status := ctx.Request().Query("status"); status != "" {
		query = query.Where("status", status)
	}
	query = services.ApplySort(query, ctx.Request().Query("sort"), []string{"created_at", "id", "status"}, "created_at")

	var archives []models.Archive
	var total int64
	if err := query.Paginate(pagination.Page, pagination.PerPage, &archives, &total); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return responses.Paginated(ctx, resources.Archives(archives, AdminArchivePrefix), pagination, total)
}

// Store queues a new archive for any client.
func (r *ArchiveController) Store(ctx http.Context) http.Response {
	var request requests.AdminCreateArchiveRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	var client models.Client
	if err := facades.Orm().Query().Where("id", request.ClientID).First(&client); err != nil || client.ID == 0 {
		return responses.InvalidField(ctx, "client_id", "client not found")
	}
	if !services.Scope(ctx).OwnsClient(&client) {
		return responses.InvalidField(ctx, "client_id", "client not found")
	}

	folders, err := r.archives.NormalizeFolders(request.Folders)
	if err != nil {
		return responses.InvalidField(ctx, "folders", "invalid folder name")
	}

	var callback *string
	if request.CallbackURL != "" {
		if err := safeurl.Validate(request.CallbackURL); err != nil {
			return responses.InvalidField(ctx, "callback_url", err.Error())
		}

		value := request.CallbackURL
		callback = &value
	}

	createdBy := uint(0)
	if id := AuthUserID(ctx); id != nil {
		createdBy = *id
	}

	archive, err := r.archives.Create(client.ID, folders, callback, models.ActorTypeUser, createdBy)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	id := archive.ID
	clientID := client.ID
	r.audit.LogFor(Actor(ctx), "archive.create", "archive", &id, &clientID, map[string]any{
		"client_id": client.ID,
		"folders":   folders,
	}, ctx.Request().Ip())

	return responses.Data(ctx, http.StatusAccepted, resources.Archive(archive, AdminArchivePrefix))
}

// Show returns one archive.
func (r *ArchiveController) Show(ctx http.Context) http.Response {
	archive, found := findArchive(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	return responses.Data(ctx, http.StatusOK, resources.Archive(archive, AdminArchivePrefix))
}

// Status returns a light payload for the polling of several archives at once.
// maxStatusIDs caps how many archives one polling call may ask about.
const maxStatusIDs = 100

func (r *ArchiveController) Status(ctx http.Context) http.Response {
	raw := ctx.Request().Query("ids")
	ids := make([]any, 0, maxStatusIDs)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id := cast.ToUint(part); id > 0 {
			ids = append(ids, id)
		}
		// The list comes straight from the query string, so it has to be
		// bounded before it becomes an IN clause.
		if len(ids) >= maxStatusIDs {
			break
		}
	}

	out := make([]map[string]any, 0)
	if len(ids) == 0 {
		return ctx.Response().Success().Json(http.Json{"data": out})
	}

	var archives []models.Archive
	if err := services.Scope(ctx).ApplyToClientColumn(
		facades.Orm().Query().Model(&models.Archive{}), "client_id",
	).WhereIn("id", ids).Get(&archives); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	for index := range archives {
		rendered := resources.Archive(&archives[index], AdminArchivePrefix)
		out = append(out, map[string]any{
			"id":         rendered["id"],
			"status":     rendered["status"],
			"progress":   rendered["progress"],
			"url":        rendered["url"],
			"expires_at": rendered["expires_at"],
		})
	}

	return ctx.Response().Success().Json(http.Json{"data": out})
}

// Download streams the generated zip file.
func (r *ArchiveController) Download(ctx http.Context) http.Response {
	archive, found := findArchive(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	return apicontrollers.DownloadArchive(ctx, archive)
}

// Destroy removes an archive and its zip file.
func (r *ArchiveController) Destroy(ctx http.Context) http.Response {
	archive, found := findArchive(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	if err := r.archives.Delete(archive); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	id := archive.ID
	clientID := archive.ClientID
	r.audit.LogFor(Actor(ctx), "archive.delete", "archive", &id, &clientID, map[string]any{}, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

func findArchive(ctx http.Context) (*models.Archive, bool) {
	id := ctx.Request().RouteInt("id")
	if id <= 0 {
		return nil, false
	}

	var archive models.Archive
	if err := facades.Orm().Query().Where("id", id).First(&archive); err != nil || archive.ID == 0 {
		return nil, false
	}

	if !services.Scope(ctx).MayUseClient(archive.ClientID) {
		return nil, false
	}

	return &archive, true
}
