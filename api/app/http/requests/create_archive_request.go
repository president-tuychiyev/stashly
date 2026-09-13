// Package requests holds the form request structs used for validation.
package requests

import (
	"github.com/goravel/framework/contracts/http"
)

// CreateArchiveRequest is the body of POST /api/archives.
type CreateArchiveRequest struct {
	Folders     []string `form:"folders" json:"folders"`
	CallbackURL string   `form:"callback_url" json:"callback_url"`
}

func (r *CreateArchiveRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *CreateArchiveRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"folders":      "array",
		"callback_url": "full_url",
	}
}
