package bootstrap

import (
	"github.com/goravel/framework/auth"
	"github.com/goravel/framework/cache"
	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/crypt"
	"github.com/goravel/framework/database"
	"github.com/goravel/framework/filesystem"
	"github.com/goravel/framework/hash"
	"github.com/goravel/framework/http"
	"github.com/goravel/framework/log"
	"github.com/goravel/framework/mail"
	"github.com/goravel/framework/queue"
	"github.com/goravel/framework/route"
	"github.com/goravel/framework/schedule"
	"github.com/goravel/framework/translation"
	"github.com/goravel/framework/validation"
	"github.com/goravel/framework/view"
	"github.com/goravel/gin"
	"github.com/goravel/postgres"
)

func Providers() []foundation.ServiceProvider {
	return []foundation.ServiceProvider{
		&log.ServiceProvider{},
		&cache.ServiceProvider{},
		&hash.ServiceProvider{},
		&http.ServiceProvider{},
		&filesystem.ServiceProvider{},
		&validation.ServiceProvider{},
		&view.ServiceProvider{},
		&route.ServiceProvider{},
		&gin.ServiceProvider{},
		&database.ServiceProvider{},
		&postgres.ServiceProvider{},
		&auth.ServiceProvider{},
		&crypt.ServiceProvider{},
		&queue.ServiceProvider{},
		&mail.ServiceProvider{},
		&translation.ServiceProvider{},
		&schedule.ServiceProvider{},
	}
}
