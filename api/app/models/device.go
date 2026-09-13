package models

import (
	"github.com/goravel/framework/database/orm"
	"github.com/goravel/framework/support/carbon"
)

type Device struct {
	orm.Model
	UID        string           `json:"uid"`
	FcmToken   *string          `json:"fcm_token"`
	Platform   string           `json:"platform"`
	AppVersion *string          `json:"app_version"`
	IP         *string          `json:"ip" gorm:"column:ip"`
	LastSeenAt *carbon.DateTime `json:"last_seen_at"`
	UserID     *uint            `json:"user_id"`
	User       *User            `json:"user,omitempty" gorm:"foreignKey:UserID"`
	ClientID   uint             `json:"client_id"`
	Client     *Client          `json:"client,omitempty" gorm:"foreignKey:ClientID"`
}
