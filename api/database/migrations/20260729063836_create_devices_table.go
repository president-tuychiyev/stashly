package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260729063836CreateDevicesTable struct{}

// Signature The unique signature for the migration.
func (r *M20260729063836CreateDevicesTable) Signature() string {
	return "20260729063836_create_devices_table"
}

// Up Run the migrations.
func (r *M20260729063836CreateDevicesTable) Up() error {
	if !facades.Schema().HasTable("devices") {
		return facades.Schema().Create("devices", func(table schema.Blueprint) {
			table.ID()
			table.String("uid", 255)
			table.String("fcm_token", 255).Nullable()
			table.Unique("fcm_token")
			table.Enum("platform", []any{"android", "ios", "web", "unknown"}).Default("unknown")
			table.String("app_version", 50).Nullable()
			table.String("ip", 45).Nullable()
			table.TimestampTz("last_seen_at").Nullable()
			table.UnsignedBigInteger("user_id").Nullable()
			table.Foreign("user_id").References("id").On("users").NullOnDelete()
			table.UnsignedBigInteger("client_id")
			table.Foreign("client_id").References("id").On("clients").CascadeOnDelete()
			table.Unique("uid", "client_id")
			table.TimestampsTz()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20260729063836CreateDevicesTable) Down() error {
	return facades.Schema().DropIfExists("devices")
}
