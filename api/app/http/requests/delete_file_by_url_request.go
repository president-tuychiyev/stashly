package requests

import (
	"github.com/goravel/framework/contracts/http"
)

type DeleteFileByURLRequest struct {
	URL string `form:"url" json:"url"`
}

func (r *DeleteFileByURLRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *DeleteFileByURLRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"url": "required|string|max_len:1000",
	}
}
