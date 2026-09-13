package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260729063825CreateClientsTable struct{}

// Signature The unique signature for the migration.
func (r *M20260729063825CreateClientsTable) Signature() string {
	return "20260729063825_create_clients_table"
}

// Up Run the migrations.
func (r *M20260729063825CreateClientsTable) Up() error {
	if !facades.Schema().HasTable("clients") {
		return facades.Schema().Create("clients", func(table schema.Blueprint) {
			table.ID()
			table.String("name", 255).Nullable()
			table.String("username", 255).Nullable()
			table.Unique("username")
			table.String("password", 255)
			table.SoftDeletesTz()
			table.Enum("status", []any{"active", "inactive", "blocked"}).Default("active")
			table.UnsignedBigInteger("creator_id").Nullable()
			table.Foreign("creator_id").References("id").On("users").NullOnDelete()
			table.UnsignedBigInteger("updater_id").Nullable()
			table.Foreign("updater_id").References("id").On("users").NullOnDelete()
			table.TimestampsTz()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20260729063825CreateClientsTable) Down() error {
	return facades.Schema().DropIfExists("clients")
}
