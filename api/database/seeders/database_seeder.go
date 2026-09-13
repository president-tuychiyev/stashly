package seeders

import (
	"github.com/goravel/framework/contracts/database/seeder"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type DatabaseSeeder struct{}

func (r *DatabaseSeeder) Signature() string {
	return "DatabaseSeeder"
}

func (r *DatabaseSeeder) Run() error {
	return facades.Seeder().Call([]seeder.Seeder{
		&RoleSeeder{},
		&UserSeeder{},
		&ClientSeeder{},
	})
}
