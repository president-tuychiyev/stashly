package requests

import (
	"github.com/goravel/framework/contracts/http"
)

// LoginRequest is the body of POST /admin/auth/login.
type LoginRequest struct {
	Email    string `form:"email" json:"email"`
	Password string `form:"password" json:"password"`
}

func (r *LoginRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *LoginRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"email":    "required|email|max_len:255",
		"password": "required|string",
	}
}

// EmailOnlyRequest is the body of the endpoints that only take an address:
// verify/resend and forgot.
type EmailOnlyRequest struct {
	Email string `form:"email" json:"email"`
}

func (r *EmailOnlyRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *EmailOnlyRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"email": "required|email|max_len:255",
	}
}

// CodePasswordRequest is the body of POST /admin/auth/verify and
// POST /admin/auth/reset: an address, the one time code and the new password.
// The password policy itself is checked by ValidatePassword, so that both
// endpoints and the profile answer with the same message.
type CodePasswordRequest struct {
	Email                string `form:"email" json:"email"`
	Code                 string `form:"code" json:"code"`
	Password             string `form:"password" json:"password"`
	PasswordConfirmation string `form:"password_confirmation" json:"password_confirmation"`
}

func (r *CodePasswordRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *CodePasswordRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"email":    "required|email|max_len:255",
		"code":     "required|string|len:6",
		"password": "required|string|max_len:255",
	}
}
