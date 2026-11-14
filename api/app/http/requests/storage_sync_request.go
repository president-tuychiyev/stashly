package requests

import (
	"github.com/goravel/framework/contracts/http"
)

type StorageSyncRequest struct {
	Orphans string `form:"orphans" json:"orphans"`
}

func (r *StorageSyncRequest) Authorize(ctx http.Context) error {
	return nil
}

func (r *StorageSyncRequest) Rules(ctx http.Context) map[string]any {
	return map[string]any{
		"orphans": "in:delete,register",
	}
}
