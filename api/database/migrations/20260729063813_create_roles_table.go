package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260729063813CreateRolesTable struct{}

// Signature The unique signature for the migration.
func (r *M20260729063813CreateRolesTable) Signature() string {
	return "20260729063813_create_roles_table"
}

// Up Run the migrations.
func (r *M20260729063813CreateRolesTable) Up() error {
	if !facades.Schema().HasTable("roles") {
		return facades.Schema().Create("roles", func(table schema.Blueprint) {
			table.ID()
			table.String("name", 255)
			table.String("slug", 255)
			table.Unique("slug")
			table.Jsonb("permissions").Default("[]")
			table.UnsignedBigInteger("creator_id").Nullable()
			table.UnsignedBigInteger("updater_id").Nullable()
			table.Boolean("is_active").Default(true)
			table.TimestampsTz()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20260729063813CreateRolesTable) Down() error {
	return facades.Schema().DropIfExists("roles")
}
