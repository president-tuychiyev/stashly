package requests

import (
	"github.com/goravel/framework/contracts/http"
)

// CreateClientRequest is the body of POST /admin/clients.
type CreateClientRequest struct {
	Name         string   `form:"name" json:"name"`
	Username     string   `form:"username" json:"username"`
	Password     string   `form:"password" json:"password"`
	Status       string   `form:"status" json:"status"`
	QuotaBytes   *int64   `form:"quota_bytes" json:"quota_bytes"`
	AllowedMimes []string `form:"allowed_mimes" json:"allowed_mimes"`
	MaxFileSize  *int64   `form:"max_file_size" json:"max_file_size"`
	// OwnerID is only honoured for a super admin; an admin always owns what it
	// creates.
	OwnerID *uint `form:"owner_id" json:"owner_id"`
}

func (r *CreateClientRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *CreateClientRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"name":     "required|string|max_len:255",
		"username": "required|string|min_len:3|max_len:255",
		"password": "string|min_len:8|max_len:255",
		"status":   "in:active,inactive,blocked",
	}
}

// UpdateClientRequest is the body of PUT /admin/clients/{id}, every field is
// optional.
type UpdateClientRequest struct {
	Name         *string   `form:"name" json:"name"`
	Username     *string   `form:"username" json:"username"`
	Status       *string   `form:"status" json:"status"`
	QuotaBytes   *int64    `form:"quota_bytes" json:"quota_bytes"`
	AllowedMimes *[]string `form:"allowed_mimes" json:"allowed_mimes"`
	MaxFileSize  *int64    `form:"max_file_size" json:"max_file_size"`
	// OwnerID is only honoured for a super admin.
	OwnerID *uint `form:"owner_id" json:"owner_id"`
}

func (r *UpdateClientRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *UpdateClientRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"name":     "string|max_len:255",
		"username": "string|min_len:3|max_len:255",
		"status":   "in:active,inactive,blocked",
	}
}

// ResetPasswordRequest is the body of POST /admin/clients/{id}/reset-password.
type ResetPasswordRequest struct {
	Password string `form:"password" json:"password"`
}

func (r *ResetPasswordRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *ResetPasswordRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"password": "string|min_len:8|max_len:255",
	}
}

// BulkDeleteRequest is the body of POST /admin/files/bulk-delete.
type BulkDeleteRequest struct {
	IDs []uint `form:"ids" json:"ids"`
}

func (r *BulkDeleteRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *BulkDeleteRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"ids": "required|array",
	}
}
