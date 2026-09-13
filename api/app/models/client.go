package models

import (
	"github.com/goravel/framework/database/orm"
)

type Client struct {
	orm.Model
	Name     *string `json:"name"`
	Username *string `json:"username"`
	Password string  `json:"-"`
	Status   string  `json:"status"`
	// QuotaBytes is the total storage the client may use. Nil means unlimited.
	QuotaBytes *int64 `json:"quota_bytes"`
	// AllowedMimes restricts uploads to the listed MIME types. An empty list
	// allows everything. Entries may use the "type/*" wildcard form.
	AllowedMimes []string `json:"allowed_mimes" gorm:"serializer:json"`
	// MaxFileSize overrides the global MAX_FILE_SIZE for this client. Nil means
	// the global value is used.
	MaxFileSize *int64 `json:"max_file_size"`
	// OwnerID is the admin user that manages this client. Admins only see the
	// clients they own; super admins see all of them.
	OwnerID   *uint `json:"owner_id"`
	Owner     *User `json:"owner,omitempty" gorm:"foreignKey:OwnerID"`
	CreatorID *uint `json:"creator_id"`
	Creator   *User `json:"creator,omitempty" gorm:"foreignKey:CreatorID"`
	UpdaterID *uint `json:"updater_id"`
	Updater   *User `json:"updater,omitempty" gorm:"foreignKey:UpdaterID"`
	orm.SoftDeletes
}
