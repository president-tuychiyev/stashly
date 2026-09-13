package api

import (
	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type MeController struct {
	storage *services.StorageService
}

func NewMeController() *MeController {
	return &MeController{storage: services.NewStorageService()}
}

// Show returns the calling client with its current storage usage.
func (r *MeController) Show(ctx http.Context) http.Response {
	client, err := CurrentClient(ctx)
	if err != nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	used, err := r.storage.UsedBytes(client.ID)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	count, err := facades.Orm().Query().Model(&models.File{}).Where("client_id", client.ID).Count()
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return responses.Data(ctx, http.StatusOK, resources.Client(client, used, count))
}
