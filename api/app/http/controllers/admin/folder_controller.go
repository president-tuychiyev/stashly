package admin

import (
	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type FolderController struct {
	storage *services.StorageService
}

func NewFolderController() *FolderController {
	return &FolderController{storage: services.NewStorageService()}
}

// Index lists the folders of one client.
func (r *FolderController) Index(ctx http.Context) http.Response {
	clientID := ctx.Request().QueryInt("client_id", 0)
	if clientID <= 0 {
		return responses.InvalidField(ctx, "client_id", "client_id is required")
	}

	if !services.Scope(ctx).MayUseClient(uint(clientID)) {
		return responses.NotFound(ctx)
	}

	parent := ""
	if raw := ctx.Request().Query("parent"); raw != "" {
		clean, err := r.storage.NormalizeFolder(raw)
		if err != nil {
			return responses.InvalidField(ctx, "parent", "invalid folder name")
		}
		parent = clean
	}

	folders, err := r.storage.Folders(uint(clientID), parent)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return responses.Data(ctx, http.StatusOK, folders)
}
