package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260729063801CreateUsersTable struct{}

// Signature The unique signature for the migration.
func (r *M20260729063801CreateUsersTable) Signature() string {
	return "20260729063801_create_users_table"
}

// Up Run the migrations.
func (r *M20260729063801CreateUsersTable) Up() error {
	if !facades.Schema().HasTable("users") {
		return facades.Schema().Create("users", func(table schema.Blueprint) {
			table.ID()
			table.String("name", 255)
			table.String("avatar_src", 500).Nullable()
			table.String("phone", 13)
			table.Unique("phone")
			table.String("email", 255).Nullable()
			table.Unique("email")
			table.String("hemis_id", 255).Nullable()
			table.Unique("hemis_id")
			table.String("password", 255).Nullable()
			table.Timestamp("birthday").Nullable()
			table.Jsonb("details").Default("{}")
			table.UnsignedBigInteger("role_id").Nullable()
			table.Foreign("role_id").References("id").On("roles").NullOnDelete()
			table.TimestampsTz()
			table.SoftDeletesTz()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20260729063801CreateUsersTable) Down() error {
	return facades.Schema().DropIfExists("users")
}
