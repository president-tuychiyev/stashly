package models

import (
	"github.com/goravel/framework/database/orm"
)

const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

type File struct {
	orm.Model
	ClientID uint    `json:"client_id"`
	Client   *Client `json:"client,omitempty" gorm:"foreignKey:ClientID"`
	// Folder is the logical folder inside the client root, "" for the root.
	Folder string `json:"folder"`
	// Name is the generated on-disk file name (uuid + extension).
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
	// Path is relative to the disk root and always starts with the client id.
	Path       string `json:"path"`
	Mime       string `json:"mime"`
	Extension  string `json:"extension"`
	Size       int64  `json:"size"`
	Sha256     string `json:"sha256"`
	Visibility string `json:"visibility"`
	orm.SoftDeletes
}

// Disk returns the filesystem disk this file lives on.
func (r *File) Disk() string {
	if r.Visibility == VisibilityPrivate {
		return "private"
	}

	return "public"
}
