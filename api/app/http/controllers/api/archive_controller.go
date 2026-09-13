package api

import (
	"fmt"
	"os"
	"strings"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/support/carbon"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/requests"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
	"github.com/president-tuychiyev/stashly/api/app/support/safeurl"
)

// ClientArchivePrefix is used to build the download url of an archive.
const ClientArchivePrefix = "/api/archives"

type ArchiveController struct {
	archives *services.ArchiveService
	audit    *services.AuditService
}

func NewArchiveController() *ArchiveController {
	return &ArchiveController{
		archives: services.NewArchiveService(),
		audit:    services.NewAuditService(),
	}
}

// Store queues a new archive for the calling client.
func (r *ArchiveController) Store(ctx http.Context) http.Response {
	var request requests.CreateArchiveRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	folders, err := r.archives.NormalizeFolders(request.Folders)
	if err != nil {
		return responses.InvalidField(ctx, "folders", "invalid folder name")
	}

	client, err := CurrentClient(ctx)
	if err != nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	var callback *string
	if request.CallbackURL != "" {
		if err := safeurl.Validate(request.CallbackURL); err != nil {
			return responses.InvalidField(ctx, "callback_url", err.Error())
		}

		value := request.CallbackURL
		callback = &value
	}

	archive, err := r.archives.Create(client.ID, folders, callback, models.ActorTypeClient, client.ID)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	id := archive.ID
	r.audit.Log(services.ClientActor(client), "archive.create", "archive", &id, map[string]any{
		"folders": folders,
	}, ctx.Request().Ip())

	return responses.Data(ctx, http.StatusAccepted, resources.Archive(archive, ClientArchivePrefix))
}

// Index lists the archives of the calling client.
func (r *ArchiveController) Index(ctx http.Context) http.Response {
	pagination := responses.ReadPagination(ctx)

	query := facades.Orm().Query().Model(&models.Archive{}).Where("client_id", ClientID(ctx))
	query = services.ApplySort(query, ctx.Request().Query("sort"), []string{"created_at", "id", "status"}, "created_at")

	var archives []models.Archive
	var total int64
	if err := query.Paginate(pagination.Page, pagination.PerPage, &archives, &total); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return responses.Paginated(ctx, resources.Archives(archives, ClientArchivePrefix), pagination, total)
}

// Show returns one archive of the calling client.
func (r *ArchiveController) Show(ctx http.Context) http.Response {
	archive, found := findClientArchive(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	return responses.Data(ctx, http.StatusOK, resources.Archive(archive, ClientArchivePrefix))
}

// Download streams the generated zip file.
func (r *ArchiveController) Download(ctx http.Context) http.Response {
	archive, found := findClientArchive(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	return DownloadArchive(ctx, archive)
}

// Destroy removes an archive and its zip file.
func (r *ArchiveController) Destroy(ctx http.Context) http.Response {
	archive, found := findClientArchive(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	if err := r.archives.Delete(archive); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	client, _ := CurrentClient(ctx)
	id := archive.ID
	r.audit.Log(services.ClientActor(client), "archive.delete", "archive", &id, map[string]any{}, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

func findClientArchive(ctx http.Context) (*models.Archive, bool) {
	id := ctx.Request().RouteInt("id")
	if id <= 0 {
		return nil, false
	}

	var archive models.Archive
	if err := facades.Orm().Query().Where("id", id).Where("client_id", ClientID(ctx)).First(&archive); err != nil || archive.ID == 0 {
		return nil, false
	}

	return &archive, true
}

// DownloadArchive streams a finished zip file, or reports 404 when the archive
// is not ready or has already expired.
func DownloadArchive(ctx http.Context, archive *models.Archive) http.Response {
	if archive.Status != models.ArchiveStatusDone || archive.Path == nil {
		return responses.NotFound(ctx)
	}
	if archive.ExpiresAt != nil && archive.ExpiresAt.StdTime().Before(carbon.Now().StdTime()) {
		return responses.NotFound(ctx)
	}

	storage := services.NewStorageService()
	target, err := storage.ResolvePath("archives", strings.TrimPrefix(*archive.Path, "archives/"))
	if err != nil {
		return responses.NotFound(ctx)
	}
	if _, err := os.Stat(target); err != nil {
		return responses.NotFound(ctx)
	}

	ctx.Response().Header("Content-Disposition", services.ContentDisposition(fmt.Sprintf("archive-%d.zip", archive.ID)))
	ctx.Response().Header("Content-Type", "application/zip")

	return ctx.Response().File(target)
}
