package models

import (
	"github.com/goravel/framework/support/carbon"
)

const (
	// OtpPurposeVerify activates an invited user and sets its first password.
	OtpPurposeVerify = "verify"
	// OtpPurposeEmailChange confirms a new address for an existing user.
	OtpPurposeEmailChange = "email_change"
	// OtpPurposePasswordReset lets a user set a new password.
	OtpPurposePasswordReset = "password_reset"
)

// OtpCode is one issued one time code. Only the keyed digest of the code is
// stored, so a database dump never reveals a usable code.
type OtpCode struct {
	ID    uint   `json:"id" gorm:"primaryKey"`
	Email string `json:"email"`
	// Purpose is one of the OtpPurpose* constants.
	Purpose string `json:"purpose"`
	// UserID is the account the code was issued for. An email change is stored
	// under the new address, so the address alone does not identify the user;
	// this column does, and it is indexed.
	UserID    *uint            `json:"user_id"`
	CodeHash  string           `json:"-"`
	ExpiresAt *carbon.DateTime `json:"expires_at"`
	Attempts  int16            `json:"attempts"`
	// ConsumedAt is set once the code has been used or invalidated.
	ConsumedAt *carbon.DateTime `json:"consumed_at"`
	Meta       map[string]any   `json:"meta" gorm:"serializer:json"`
	CreatedAt  *carbon.DateTime `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

func (r *OtpCode) TableName() string {
	return "otp_codes"
}
