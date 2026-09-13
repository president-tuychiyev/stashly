package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260914000004AddClientIDToAuditLogsTable struct{}

// Signature The unique signature for the migration.
func (r *M20260914000004AddClientIDToAuditLogsTable) Signature() string {
	return "20260914000004_add_client_id_to_audit_logs_table"
}

// Up records which client an audited action belongs to, so the audit listing
// can be scoped to the clients an admin owns without joining every subject
// table in turn.
func (r *M20260914000004AddClientIDToAuditLogsTable) Up() error {
	if !facades.Schema().HasTable("audit_logs") || facades.Schema().HasColumn("audit_logs", "client_id") {
		return nil
	}

	if err := facades.Schema().Table("audit_logs", func(table schema.Blueprint) {
		table.UnsignedBigInteger("client_id").Nullable()
		table.Index("client_id")
	}); err != nil {
		return err
	}

	// Backfill what can be derived without guessing: rows about a client, and
	// rows a client itself produced.
	if err := facades.Schema().Sql(
		`UPDATE audit_logs SET client_id = subject_id WHERE client_id IS NULL AND subject_type = 'client'`,
	); err != nil {
		return err
	}

	return facades.Schema().Sql(
		`UPDATE audit_logs SET client_id = actor_id WHERE client_id IS NULL AND actor_type = 'client'`,
	)
}

// Down Reverse the migrations.
func (r *M20260914000004AddClientIDToAuditLogsTable) Down() error {
	if !facades.Schema().HasColumn("audit_logs", "client_id") {
		return nil
	}

	return facades.Schema().Table("audit_logs", func(table schema.Blueprint) {
		table.DropIndex("client_id")
		table.DropColumn("client_id")
	})
}
