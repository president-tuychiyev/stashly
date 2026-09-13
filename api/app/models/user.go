package models

import (
	"github.com/goravel/framework/database/orm"
	"github.com/goravel/framework/support/carbon"
)

const (
	// UserStatusPending is an invited user that has not verified its email and
	// has no password yet.
	UserStatusPending = "pending"
	// UserStatusActive is a user that may sign in.
	UserStatusActive = "active"
	// UserStatusBlocked is a user that is refused at login.
	UserStatusBlocked = "blocked"
)

type User struct {
	orm.Model
	Name      string  `json:"name"`
	AvatarSrc *string `json:"avatar_src"`
	// Email is the login identity and is unique across live users.
	Email           string           `json:"email"`
	Password        *string          `json:"-"`
	Status          string           `json:"status"`
	LastLoginAt     *carbon.DateTime `json:"last_login_at"`
	EmailVerifiedAt *carbon.DateTime `json:"email_verified_at"`
	// CredentialsChangedAt is when the account's credentials last changed: a
	// password reset or change, an email change, or a block. Any token issued
	// before it is refused, which is how a session is ended without a
	// revocation list.
	CredentialsChangedAt *carbon.DateTime `json:"-"`
	RoleID               *uint            `json:"role_id"`
	Role                 *Role            `json:"role,omitempty" gorm:"foreignKey:RoleID"`
	Devices              []Device         `json:"devices,omitempty" gorm:"foreignKey:UserID"`
	orm.SoftDeletes
}
