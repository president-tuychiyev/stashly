package models

import (
	"github.com/goravel/framework/database/orm"
	"github.com/goravel/framework/support/carbon"
)

const (
	ArchiveStatusPending    = "pending"
	ArchiveStatusProcessing = "processing"
	ArchiveStatusDone       = "done"
	ArchiveStatusFailed     = "failed"
	ArchiveStatusExpired    = "expired"

	ActorTypeUser   = "user"
	ActorTypeClient = "client"
	ActorTypeSystem = "system"
)

type Archive struct {
	orm.Model
	ClientID uint    `json:"client_id"`
	Client   *Client `json:"client,omitempty" gorm:"foreignKey:ClientID"`
	// CreatedByType is either "user" (admin) or "client".
	CreatedByType string `json:"created_by_type"`
	CreatedByID   uint   `json:"created_by_id"`
	// Folders lists the client folders to zip. Empty or ["*"] means everything.
	Folders     []string         `json:"folders" gorm:"serializer:json"`
	Status      string           `json:"status"`
	Progress    int              `json:"progress"`
	FilesCount  int              `json:"files_count"`
	Size        *int64           `json:"size"`
	Path        *string          `json:"path"`
	Error       *string          `json:"error"`
	CallbackURL *string          `json:"callback_url" gorm:"column:callback_url"`
	ExpiresAt   *carbon.DateTime `json:"expires_at"`
	FinishedAt  *carbon.DateTime `json:"finished_at"`
}
