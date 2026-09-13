package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260912000004CreateAuditLogsTable struct{}

// Signature The unique signature for the migration.
func (r *M20260912000004CreateAuditLogsTable) Signature() string {
	return "20260912000004_create_audit_logs_table"
}

// Up Run the migrations.
func (r *M20260912000004CreateAuditLogsTable) Up() error {
	if facades.Schema().HasTable("audit_logs") {
		return nil
	}

	return facades.Schema().Create("audit_logs", func(table schema.Blueprint) {
		table.ID()
		table.Enum("actor_type", []any{"user", "client", "system"})
		table.UnsignedBigInteger("actor_id").Nullable()
		table.String("actor_name", 255).Nullable()
		table.String("action", 100)
		table.Index("action")
		table.String("subject_type", 50).Nullable()
		table.UnsignedBigInteger("subject_id").Nullable()
		table.Jsonb("details").Default("{}")
		table.String("ip", 45).Nullable()
		table.TimestampTz("created_at").UseCurrent()
	})
}

// Down Reverse the migrations.
func (r *M20260912000004CreateAuditLogsTable) Down() error {
	return facades.Schema().DropIfExists("audit_logs")
}
