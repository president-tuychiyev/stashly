package feature

import (
	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

// facadesOrmFirst loads the first file of a client.
func facadesOrmFirst(dest *models.File, clientID uint) error {
	return facades.Orm().Query().Where("client_id", clientID).OrderBy("id").First(dest)
}

// facadesOrmCount counts the files of a client.
func facadesOrmCount(clientID uint) (int64, error) {
	return facades.Orm().Query().Model(&models.File{}).Where("client_id", clientID).Count()
}
