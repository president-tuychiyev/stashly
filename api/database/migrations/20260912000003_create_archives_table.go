package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260912000003CreateArchivesTable struct{}

// Signature The unique signature for the migration.
func (r *M20260912000003CreateArchivesTable) Signature() string {
	return "20260912000003_create_archives_table"
}

// Up Run the migrations.
func (r *M20260912000003CreateArchivesTable) Up() error {
	if facades.Schema().HasTable("archives") {
		return nil
	}

	return facades.Schema().Create("archives", func(table schema.Blueprint) {
		table.ID()
		table.UnsignedBigInteger("client_id")
		table.Foreign("client_id").References("id").On("clients").CascadeOnDelete()
		table.Enum("created_by_type", []any{"user", "client"})
		table.UnsignedBigInteger("created_by_id")
		table.Jsonb("folders").Default("[]")
		table.Enum("status", []any{"pending", "processing", "done", "failed", "expired"}).Default("pending")
		table.SmallInteger("progress").Default(0)
		table.Integer("files_count").Default(0)
		table.BigInteger("size").Nullable()
		table.String("path", 500).Nullable()
		table.Text("error").Nullable()
		table.String("callback_url", 1000).Nullable()
		table.TimestampTz("expires_at").Nullable()
		table.TimestampTz("finished_at").Nullable()
		table.TimestampsTz()
		table.Index("status")
		table.Index("expires_at")
	})
}

// Down Reverse the migrations.
func (r *M20260912000003CreateArchivesTable) Down() error {
	return facades.Schema().DropIfExists("archives")
}
