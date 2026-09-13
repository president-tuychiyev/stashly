package migrations

import (
	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260913000001PartialUniqueUsernameOnClientsTable struct{}

// Signature The unique signature for the migration.
func (r *M20260913000001PartialUniqueUsernameOnClientsTable) Signature() string {
	return "20260913000001_partial_unique_username_on_clients_table"
}

// Up replaces the plain unique index on clients.username with a partial one
// that ignores soft deleted rows.
//
// Clients are soft deleted, so the row of a deleted client stays in the table
// and its username stays taken forever: recreating a client under the same
// name failed with a raw 500 from the database. A partial unique index keeps
// the guarantee that matters (no two live clients share a username) while
// letting the name be reused once the old client is in the trash.
func (r *M20260913000001PartialUniqueUsernameOnClientsTable) Up() error {
	// Depending on how the column was declared the constraint may exist as a
	// table constraint, as a bare index, or not at all, so both forms are
	// dropped and neither is required to be there.
	if err := facades.Schema().Sql(`ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_username_unique`); err != nil {
		return err
	}
	if err := facades.Schema().Sql(`DROP INDEX IF EXISTS clients_username_unique`); err != nil {
		return err
	}

	return facades.Schema().Sql(
		`CREATE UNIQUE INDEX IF NOT EXISTS clients_username_active_unique ON clients (username) WHERE deleted_at IS NULL`,
	)
}

// Down restores the unconditional unique index.
func (r *M20260913000001PartialUniqueUsernameOnClientsTable) Down() error {
	if err := facades.Schema().Sql(`DROP INDEX IF EXISTS clients_username_active_unique`); err != nil {
		return err
	}

	return facades.Schema().Sql(
		`CREATE UNIQUE INDEX IF NOT EXISTS clients_username_unique ON clients (username)`,
	)
}
