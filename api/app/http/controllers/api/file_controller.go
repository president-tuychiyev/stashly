package api

import (
	"os"

	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type FileController struct {
	storage *services.StorageService
	audit   *services.AuditService
}

func NewFileController() *FileController {
	return &FileController{storage: services.NewStorageService(), audit: services.NewAuditService()}
}

// Store accepts one file under "file" or several under "files[]".
func (r *FileController) Store(ctx http.Context) http.Response {
	client, err := CurrentClient(ctx)
	if err != nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	stored, uploadErr := services.NewUploadService().Handle(ctx, client, services.ClientActor(client))
	if uploadErr != nil {
		return UploadFailure(ctx, uploadErr)
	}

	return responses.Data(ctx, http.StatusCreated, resources.Files(stored))
}

// Index lists the files of the current client.
func (r *FileController) Index(ctx http.Context) http.Response {
	clientID := ClientID(ctx)
	pagination := responses.ReadPagination(ctx)

	query := facades.Orm().Query().Model(&models.File{}).Where("client_id", clientID)

	if folder := ctx.Request().Query("folder"); folder != "" {
		clean, folderErr := r.storage.NormalizeFolder(folder)
		if folderErr != nil {
			return responses.InvalidField(ctx, "folder", "invalid folder name")
		}
		query = query.Where("folder", clean)
	}

	if search := ctx.Request().Query("search"); search != "" {
		query = query.Where("original_name ILIKE ?", "%"+services.EscapeLike(search)+"%")
	}

	query = services.ApplySort(query, ctx.Request().Query("sort"), fileSortColumns, "created_at")

	var files []models.File
	var total int64
	if err := query.Paginate(pagination.Page, pagination.PerPage, &files, &total); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return responses.Paginated(ctx, resources.Files(files), pagination, total)
}

// Show returns a single file owned by the current client.
func (r *FileController) Show(ctx http.Context) http.Response {
	file, found := findClientFile(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	return responses.Data(ctx, http.StatusOK, resources.File(file))
}

// Download streams a file, public or private, as an attachment.
func (r *FileController) Download(ctx http.Context) http.Response {
	file, found := findClientFile(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	return DownloadFile(ctx, file)
}

// Destroy removes a file from disk and from the database.
func (r *FileController) Destroy(ctx http.Context) http.Response {
	file, found := findClientFile(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	if err := r.storage.Delete(file); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	client, _ := CurrentClient(ctx)
	id := file.ID
	r.audit.Log(services.ClientActor(client), "file.delete", "file", &id, map[string]any{
		"path": file.Path,
		"size": file.Size,
	}, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

// fileSortColumns are the columns a caller may sort file listings by.
var fileSortColumns = []string{"created_at", "size", "original_name", "id"}

// UploadFailure turns an upload rejection into the standard error envelope.
func UploadFailure(ctx http.Context, failure *services.UploadError) http.Response {
	if failure.Status == http.StatusUnprocessableEntity {
		return responses.Invalid(ctx, failure.Fields)
	}

	if len(failure.Fields) > 0 {
		return ctx.Response().Status(failure.Status).Json(http.Json{
			"message": failure.Message,
			"errors":  failure.Fields,
		})
	}

	return responses.Error(ctx, failure.Status, failure.Message)
}

// findClientFile loads a file by route id and makes sure it belongs to the
// calling client. Files of other clients are reported as missing on purpose.
func findClientFile(ctx http.Context) (*models.File, bool) {
	id := ctx.Request().RouteInt("id")
	if id <= 0 {
		return nil, false
	}

	var file models.File
	if err := facades.Orm().Query().Where("id", id).Where("client_id", ClientID(ctx)).First(&file); err != nil || file.ID == 0 {
		return nil, false
	}

	return &file, true
}

// DownloadFile sends a stored file as an attachment with a safe file name.
func DownloadFile(ctx http.Context, file *models.File) http.Response {
	storage := services.NewStorageService()

	target, err := storage.AbsolutePath(file)
	if err != nil {
		return responses.NotFound(ctx)
	}
	if _, err := os.Stat(target); err != nil {
		return responses.NotFound(ctx)
	}

	ctx.Response().Header("Content-Disposition", services.ContentDisposition(file.OriginalName))
	ctx.Response().Header("Content-Type", file.Mime)

	return ctx.Response().File(target)
}
