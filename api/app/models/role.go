package models

import (
	"github.com/goravel/framework/database/orm"
)

type Role struct {
	orm.Model
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Permissions []string `json:"permissions" gorm:"serializer:json"`
	CreatorID   *uint    `json:"creator_id"`
	Creator     *User    `json:"creator,omitempty" gorm:"foreignKey:CreatorID"`
	UpdaterID   *uint    `json:"updater_id"`
	Updater     *User    `json:"updater,omitempty" gorm:"foreignKey:UpdaterID"`
	IsActive    bool     `json:"is_active"`
}
