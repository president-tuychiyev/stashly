package requests

import (
	"github.com/goravel/framework/contracts/http"
)

// AdminCreateArchiveRequest is the body of POST /admin/archives.
type AdminCreateArchiveRequest struct {
	ClientID    uint     `form:"client_id" json:"client_id"`
	Folders     []string `form:"folders" json:"folders"`
	CallbackURL string   `form:"callback_url" json:"callback_url"`
}

func (r *AdminCreateArchiveRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *AdminCreateArchiveRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"client_id":    "required|uint",
		"folders":      "array",
		"callback_url": "full_url",
	}
}
