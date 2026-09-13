package requests

import (
	"github.com/goravel/framework/contracts/http"
)

// CreateUserRequest is the body of POST /admin/users.
type CreateUserRequest struct {
	Name  string `form:"name" json:"name"`
	Email string `form:"email" json:"email"`
	Role  string `form:"role" json:"role"`
}

func (r *CreateUserRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *CreateUserRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"name":  "required|string|min_len:2|max_len:255",
		"email": "required|email|max_len:255",
		"role":  "required|in:admin,super_admin",
	}
}

// UpdateUserRequest is the body of PUT /admin/users/{id}, every field optional.
type UpdateUserRequest struct {
	Name   *string `form:"name" json:"name"`
	Role   *string `form:"role" json:"role"`
	Status *string `form:"status" json:"status"`
}

func (r *UpdateUserRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *UpdateUserRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"name":   "string|min_len:2|max_len:255",
		"role":   "in:admin,super_admin",
		"status": "in:active,blocked",
	}
}
