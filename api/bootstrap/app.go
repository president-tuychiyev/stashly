package bootstrap

import (
	contractsfoundation "github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/foundation"

	"github.com/president-tuychiyev/stashly/api/config"
	"github.com/president-tuychiyev/stashly/api/routes"
)

func Boot() contractsfoundation.Application {
	return foundation.Setup().
		WithMigrations(Migrations).
		WithSeeders(Seeders).
		WithJobs(Jobs).
		WithCommands(Commands).
		WithSchedule(Schedule).
		WithRouting(func() {
			routes.Api()
		}).
		WithProviders(Providers).
		WithConfig(config.Boot).
		WithCallback(func() {
			// The configuration is checked before anything else runs, so a
			// deployment missing a secret fails at boot instead of serving.
			CheckSecrets()
			RecoverArchives()
		}).
		Create()
}
