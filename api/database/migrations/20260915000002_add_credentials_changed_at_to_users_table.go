package migrations

import (
	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260915000002AddCredentialsChangedAtToUsersTable struct{}

// Signature The unique signature for the migration.
func (r *M20260915000002AddCredentialsChangedAtToUsersTable) Signature() string {
	return "20260915000002_add_credentials_changed_at_to_users_table"
}

// Up adds the timestamp that invalidates tokens issued before a credential
// change.
//
// goravel's JWT payload is fixed (key and subject only), so nothing can be
// carried inside the token itself; instead every request compares the token's
// "iat" claim against this column and refuses a token that predates the last
// password reset, password change, email change or block. Existing rows are
// stamped with now(), so every token handed out before this migration is
// rejected once: there is no way to tell which of them were still wanted.
func (r *M20260915000002AddCredentialsChangedAtToUsersTable) Up() error {
	if !facades.Schema().HasTable("users") {
		return nil
	}

	if facades.Schema().HasColumn("users", "credentials_changed_at") {
		return nil
	}

	if err := facades.Schema().Sql(
		`ALTER TABLE users ADD COLUMN credentials_changed_at timestamptz`,
	); err != nil {
		return err
	}

	return facades.Schema().Sql(`UPDATE users SET credentials_changed_at = now()`)
}

// Down Reverse the migrations.
func (r *M20260915000002AddCredentialsChangedAtToUsersTable) Down() error {
	if !facades.Schema().HasTable("users") {
		return nil
	}

	return facades.Schema().Sql(`ALTER TABLE users DROP COLUMN IF EXISTS credentials_changed_at`)
}
