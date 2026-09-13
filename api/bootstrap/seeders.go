package bootstrap

import (
	"github.com/goravel/framework/contracts/database/seeder"

	"github.com/president-tuychiyev/stashly/api/database/seeders"
)

func Seeders() []seeder.Seeder {
	// Only the entry point is registered here: DatabaseSeeder.Run() already
	// calls RoleSeeder, UserSeeder and ClientSeeder in the right order, so
	// listing them again would run each of them twice.
	return []seeder.Seeder{
		&seeders.DatabaseSeeder{},
	}
}
