package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260914000001AlterUsersForEmailAuth struct{}

// Signature The unique signature for the migration.
func (r *M20260914000001AlterUsersForEmailAuth) Signature() string {
	return "20260914000001_alter_users_for_email_auth"
}

// Up moves the users table from phone based logins to email based ones and
// adds the lifecycle columns the invite/verify flow needs.
//
// The scaffold shipped a phone column with a unique index and a nullable
// email; from here on the email is the identity, so it has to be filled in for
// every existing row before it can be made NOT NULL. Rows that only ever had a
// phone get a synthetic "<phone>@legacy.local" address, and a row without
// either falls back to its id. Nothing is deleted.
func (r *M20260914000001AlterUsersForEmailAuth) Up() error {
	if !facades.Schema().HasTable("users") {
		return nil
	}

	if facades.Schema().HasColumn("users", "phone") {
		if err := facades.Schema().Sql(
			`UPDATE users SET email = phone || '@legacy.local' WHERE (email IS NULL OR email = '') AND phone IS NOT NULL AND phone <> ''`,
		); err != nil {
			return err
		}
	}
	if err := facades.Schema().Sql(
		`UPDATE users SET email = 'user-' || id || '@legacy.local' WHERE email IS NULL OR email = ''`,
	); err != nil {
		return err
	}

	// A duplicate email would block the unique index that is about to become
	// the login key, so keep the oldest row and push the rest aside.
	if err := facades.Schema().Sql(`
		UPDATE users SET email = email || '.dup-' || id
		WHERE id NOT IN (SELECT min(id) FROM users GROUP BY lower(email))
	`); err != nil {
		return err
	}

	// Everything the blueprint needs to know is resolved before Table() is
	// called: the callback runs while the schema builder is busy, and asking
	// it another question from inside would deadlock on its own connection.
	var drop []string
	for _, column := range []string{"phone", "hemis_id", "birthday", "details"} {
		if facades.Schema().HasColumn("users", column) {
			drop = append(drop, column)
		}
	}
	addStatus := !facades.Schema().HasColumn("users", "status")
	addLastLogin := !facades.Schema().HasColumn("users", "last_login_at")
	addVerifiedAt := !facades.Schema().HasColumn("users", "email_verified_at")

	if err := facades.Schema().Table("users", func(table schema.Blueprint) {
		// Dropping the column drops the unique index that came with it.
		if len(drop) > 0 {
			table.DropColumn(drop...)
		}

		if addStatus {
			table.Enum("status", []any{"pending", "active", "blocked"}).Default("pending")
		}
		if addLastLogin {
			table.TimestampTz("last_login_at").Nullable()
		}
		if addVerifiedAt {
			table.TimestampTz("email_verified_at").Nullable()
		}
	}); err != nil {
		return err
	}

	// Everything that existed before this migration was a working login, so it
	// stays usable instead of being downgraded to "pending".
	if err := facades.Schema().Sql(
		`UPDATE users SET status = 'active', email_verified_at = COALESCE(email_verified_at, now()) WHERE status = 'pending' AND password IS NOT NULL`,
	); err != nil {
		return err
	}

	return facades.Schema().Sql(`ALTER TABLE users ALTER COLUMN email SET NOT NULL`)
}

// Down reverses the schema change, but it is LOSSY and cannot restore the
// data Up() replaced. Read this before rolling back a database you care about.
//
// What comes back: the email column is nullable again and a nullable phone
// column of the original width is recreated. It is deliberately recreated
// WITHOUT its old unique index, because the phone numbers themselves are gone
// and a unique index over a column full of NULLs would only pretend the
// constraint is back.
//
// What does not come back:
//   - phone, hemis_id, birthday and details were dropped by Up(); their values
//     are not stored anywhere else, so every one of them is lost. The
//     recreated phone column is empty for every row.
//   - status, last_login_at and email_verified_at are dropped here, so the
//     lifecycle state of every account (pending/active/blocked) is lost too. A
//     re-run of Up() marks every row with a password "active" again, which
//     silently un-blocks any account that was blocked.
//   - the synthetic "<phone>@legacy.local" and "user-<id>@legacy.local"
//     addresses Up() invented stay in the email column, as do the ".dup-<id>"
//     suffixes it appended to duplicates. Down() cannot tell them apart from
//     real addresses.
//
// Take a dump before rolling back.
func (r *M20260914000001AlterUsersForEmailAuth) Down() error {
	if !facades.Schema().HasTable("users") {
		return nil
	}

	if err := facades.Schema().Sql(`ALTER TABLE users ALTER COLUMN email DROP NOT NULL`); err != nil {
		return err
	}

	var drop []string
	for _, column := range []string{"status", "last_login_at", "email_verified_at"} {
		if facades.Schema().HasColumn("users", column) {
			drop = append(drop, column)
		}
	}
	addPhone := !facades.Schema().HasColumn("users", "phone")

	return facades.Schema().Table("users", func(table schema.Blueprint) {
		if len(drop) > 0 {
			table.DropColumn(drop...)
		}
		if addPhone {
			table.String("phone", 13).Nullable()
		}
	})
}
