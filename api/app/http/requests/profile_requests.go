package requests

import (
	"github.com/goravel/framework/contracts/http"
)

// UpdateProfileRequest is the body of PUT /admin/profile.
type UpdateProfileRequest struct {
	Name string `form:"name" json:"name"`
}

func (r *UpdateProfileRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *UpdateProfileRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"name": "required|string|min_len:2|max_len:255",
	}
}

// ChangePasswordRequest is the body of PUT /admin/profile/password.
type ChangePasswordRequest struct {
	CurrentPassword      string `form:"current_password" json:"current_password"`
	Password             string `form:"password" json:"password"`
	PasswordConfirmation string `form:"password_confirmation" json:"password_confirmation"`
}

func (r *ChangePasswordRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *ChangePasswordRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"current_password": "required|string",
		"password":         "required|string|max_len:255",
	}
}

// ChangeEmailRequest is the body of POST /admin/profile/email.
type ChangeEmailRequest struct {
	Email           string `form:"email" json:"email"`
	CurrentPassword string `form:"current_password" json:"current_password"`
}

func (r *ChangeEmailRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *ChangeEmailRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"email":            "required|email|max_len:255",
		"current_password": "required|string",
	}
}

// ConfirmEmailRequest is the body of POST /admin/profile/email/confirm.
type ConfirmEmailRequest struct {
	Code string `form:"code" json:"code"`
}

func (r *ConfirmEmailRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *ConfirmEmailRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"code": "required|string|len:6",
	}
}
