package api

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

// Index lists the direct sub folders of the optional "parent" folder.
func (r *FolderController) Index(ctx http.Context) http.Response {
	parent := ""
	if raw := ctx.Request().Query("parent"); raw != "" {
		clean, err := r.storage.NormalizeFolder(raw)
		if err != nil {
			return responses.InvalidField(ctx, "parent", "invalid folder name")
		}
		parent = clean
	}

	folders, err := r.storage.Folders(ClientID(ctx), parent)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return responses.Data(ctx, http.StatusOK, folders)
}
