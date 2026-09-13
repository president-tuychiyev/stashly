// Package api holds the controllers of the client facing surface, everything
// under the /api prefix. The api_check middleware has already authenticated
// the client and put its id in the request context.
package api

import (
	"errors"

	"github.com/goravel/framework/contracts/http"
	"github.com/spf13/cast"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

// ClientID returns the id of the client that signed the current request.
func ClientID(ctx http.Context) uint {
	return cast.ToUint(ctx.Value("client_id"))
}

// CurrentClient loads the client that signed the current request.
func CurrentClient(ctx http.Context) (*models.Client, error) {
	id := ClientID(ctx)
	if id == 0 {
		return nil, errors.New("no authenticated client in context")
	}

	var client models.Client
	if err := facades.Orm().Query().Where("id", id).First(&client); err != nil {
		return nil, err
	}
	if client.ID == 0 {
		return nil, errors.New("client not found")
	}

	return &client, nil
}
