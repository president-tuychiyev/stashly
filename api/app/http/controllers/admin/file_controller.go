package admin

import (
	"github.com/goravel/framework/contracts/http"
	"github.com/spf13/cast"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	apicontrollers "github.com/president-tuychiyev/stashly/api/app/http/controllers/api"
	"github.com/president-tuychiyev/stashly/api/app/http/requests"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type FileController struct {
	storage *services.StorageService
	uploads *services.UploadService
	audit   *services.AuditService
}

func NewFileController() *FileController {
	return &FileController{
		storage: services.NewStorageService(),
		uploads: services.NewUploadService(),
		audit:   services.NewAuditService(),
	}
}

// Index lists the files of every client.
func (r *FileController) Index(ctx http.Context) http.Response {
	pagination := responses.ReadPagination(ctx)

	query := services.Scope(ctx).ApplyToClientColumn(
		facades.Orm().Query().Model(&models.File{}).With("Client"), "client_id")

	if clientID := ctx.Request().QueryInt("client_id", 0); clientID > 0 {
		query = query.Where("client_id", clientID)
	}
	if folder := ctx.Request().Query("folder"); folder != "" {
		clean, err := r.storage.NormalizeFolder(folder)
		if err != nil {
			return responses.InvalidField(ctx, "folder", "invalid folder name")
		}
		query = query.Where("folder", clean)
	}
	if visibility := ctx.Request().Query("visibility"); visibility != "" {
		query = query.Where("visibility", visibility)
	}
	if mime := ctx.Request().Query("mime"); mime != "" {
		query = query.Where("mime ILIKE ?", services.EscapeLike(mime)+"%")
	}
	if search := ctx.Request().Query("search"); search != "" {
		query = query.Where("original_name ILIKE ?", "%"+services.EscapeLike(search)+"%")
	}

	query = services.ApplySort(query, ctx.Request().Query("sort"), []string{"created_at", "size", "original_name", "id"}, "created_at")

	var files []models.File
	var total int64
	if err := query.Paginate(pagination.Page, pagination.PerPage, &files, &total); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return responses.Paginated(ctx, resources.Files(files), pagination, total)
}

// Store uploads files on behalf of a client.
func (r *FileController) Store(ctx http.Context) http.Response {
	clientID := cast.ToUint(ctx.Request().Input("client_id"))
	if clientID == 0 {
		return responses.InvalidField(ctx, "client_id", "client_id is required")
	}

	var client models.Client
	if err := facades.Orm().Query().Where("id", clientID).First(&client); err != nil || client.ID == 0 {
		return responses.InvalidField(ctx, "client_id", "client not found")
	}
	// A client of another admin is answered exactly like one that does not
	// exist, so the upload endpoint cannot be used to probe for ids.
	if !services.Scope(ctx).OwnsClient(&client) {
		return responses.InvalidField(ctx, "client_id", "client not found")
	}

	stored, uploadErr := r.uploads.Handle(ctx, &client, Actor(ctx))
	if uploadErr != nil {
		return apicontrollers.UploadFailure(ctx, uploadErr)
	}

	return responses.Data(ctx, http.StatusCreated, resources.Files(stored))
}

// Show returns one file.
func (r *FileController) Show(ctx http.Context) http.Response {
	file, found := findFile(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	return responses.Data(ctx, http.StatusOK, resources.File(file))
}

// Download streams one file as an attachment.
func (r *FileController) Download(ctx http.Context) http.Response {
	file, found := findFile(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	return apicontrollers.DownloadFile(ctx, file)
}

// Destroy removes one file.
func (r *FileController) Destroy(ctx http.Context) http.Response {
	file, found := findFile(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	if err := r.storage.Delete(file); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	id := file.ID
	clientID := file.ClientID
	r.audit.LogFor(Actor(ctx), "file.delete", "file", &id, &clientID, map[string]any{"path": file.Path}, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

// BulkDelete removes several files in one call.
func (r *FileController) BulkDelete(ctx http.Context) http.Response {
	var request requests.BulkDeleteRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	ids := make([]any, 0, len(request.IDs))
	for _, id := range request.IDs {
		ids = append(ids, id)
	}

	var files []models.File
	if err := services.Scope(ctx).ApplyToClientColumn(
		facades.Orm().Query().Model(&models.File{}), "client_id",
	).WhereIn("id", ids).Get(&files); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	deleted := 0
	for index := range files {
		if err := r.storage.Delete(&files[index]); err != nil {
			facades.Log().Warning("bulk delete failed for a file: " + err.Error())
			continue
		}
		deleted++

		id := files[index].ID
		clientID := files[index].ClientID
		r.audit.LogFor(Actor(ctx), "file.delete", "file", &id, &clientID, map[string]any{"path": files[index].Path}, ctx.Request().Ip())
	}

	return ctx.Response().Success().Json(http.Json{"deleted": deleted})
}

// findFile loads the file of the route, but only when the caller owns the
// client it belongs to.
func findFile(ctx http.Context) (*models.File, bool) {
	id := ctx.Request().RouteInt("id")
	if id <= 0 {
		return nil, false
	}

	var file models.File
	if err := facades.Orm().Query().Where("id", id).First(&file); err != nil || file.ID == 0 {
		return nil, false
	}

	if !services.Scope(ctx).MayUseClient(file.ClientID) {
		return nil, false
	}

	return &file, true
}
