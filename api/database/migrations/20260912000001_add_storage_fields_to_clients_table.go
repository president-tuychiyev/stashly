package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260912000001AddStorageFieldsToClientsTable struct{}

// Signature The unique signature for the migration.
func (r *M20260912000001AddStorageFieldsToClientsTable) Signature() string {
	return "20260912000001_add_storage_fields_to_clients_table"
}

// Up Run the migrations.
func (r *M20260912000001AddStorageFieldsToClientsTable) Up() error {
	return facades.Schema().Table("clients", func(table schema.Blueprint) {
		if !facades.Schema().HasColumn("clients", "quota_bytes") {
			table.BigInteger("quota_bytes").Nullable()
		}
		if !facades.Schema().HasColumn("clients", "allowed_mimes") {
			table.Jsonb("allowed_mimes").Default("[]")
		}
		if !facades.Schema().HasColumn("clients", "max_file_size") {
			table.BigInteger("max_file_size").Nullable()
		}
	})
}

// Down Reverse the migrations.
func (r *M20260912000001AddStorageFieldsToClientsTable) Down() error {
	return facades.Schema().DropColumns("clients", []string{"quota_bytes", "allowed_mimes", "max_file_size"})
}
