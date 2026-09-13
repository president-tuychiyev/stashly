package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260914000002AddOwnerIDToClientsTable struct{}

// Signature The unique signature for the migration.
func (r *M20260914000002AddOwnerIDToClientsTable) Signature() string {
	return "20260914000002_add_owner_id_to_clients_table"
}

// Up adds the admin that manages a client. Existing clients are handed to the
// first super admin so that nothing becomes unreachable for the admin role.
func (r *M20260914000002AddOwnerIDToClientsTable) Up() error {
	if !facades.Schema().HasTable("clients") {
		return nil
	}

	if !facades.Schema().HasColumn("clients", "owner_id") {
		if err := facades.Schema().Table("clients", func(table schema.Blueprint) {
			table.UnsignedBigInteger("owner_id").Nullable()
			table.Index("owner_id")
			table.Foreign("owner_id").References("id").On("users").NullOnDelete()
		}); err != nil {
			return err
		}
	}

	return facades.Schema().Sql(`
		UPDATE clients SET owner_id = (
			SELECT u.id FROM users u
			LEFT JOIN roles r ON r.id = u.role_id
			WHERE r.slug = 'super_admin' AND u.deleted_at IS NULL
			ORDER BY u.id LIMIT 1
		)
		WHERE owner_id IS NULL
	`)
}

// Down Reverse the migrations.
func (r *M20260914000002AddOwnerIDToClientsTable) Down() error {
	if !facades.Schema().HasColumn("clients", "owner_id") {
		return nil
	}

	return facades.Schema().Table("clients", func(table schema.Blueprint) {
		table.DropForeign("owner_id")
		table.DropIndex("owner_id")
		table.DropColumn("owner_id")
	})
}
