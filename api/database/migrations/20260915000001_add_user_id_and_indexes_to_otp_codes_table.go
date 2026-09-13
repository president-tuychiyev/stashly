package migrations

import (
	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260915000001AddUserIDAndIndexesToOtpCodesTable struct{}

// Signature The unique signature for the migration.
func (r *M20260915000001AddUserIDAndIndexesToOtpCodesTable) Signature() string {
	return "20260915000001_add_user_id_and_indexes_to_otp_codes_table"
}

// Up gives otp_codes the user column the email change confirm endpoint needs
// and the indexes every hot query on the table wants.
//
// The partial unique index is the load bearing one: it makes two usable codes
// for the same address and purpose impossible, so the resend gap cannot be
// raced. Everything already consumed is outside the index, so the history of a
// mailbox is untouched.
func (r *M20260915000001AddUserIDAndIndexesToOtpCodesTable) Up() error {
	if !facades.Schema().HasTable("otp_codes") {
		return nil
	}

	if !facades.Schema().HasColumn("otp_codes", "user_id") {
		if err := facades.Schema().Sql(`ALTER TABLE otp_codes ADD COLUMN user_id bigint`); err != nil {
			return err
		}
	}

	// Codes issued before this migration carry the user in their meta.
	if err := facades.Schema().Sql(`
		UPDATE otp_codes
		SET user_id = (meta->>'user_id')::bigint
		WHERE user_id IS NULL
		  AND meta->>'user_id' ~ '^[0-9]+$'
	`); err != nil {
		return err
	}

	// The unique index below cannot be created while duplicates are still
	// outstanding, and a duplicate is by definition a code that must not be
	// usable any more: keep the newest per (email, purpose), consume the rest.
	if err := facades.Schema().Sql(`
		UPDATE otp_codes SET consumed_at = now()
		WHERE consumed_at IS NULL
		  AND id NOT IN (
			SELECT max(id) FROM otp_codes WHERE consumed_at IS NULL GROUP BY email, purpose
		  )
	`); err != nil {
		return err
	}

	for _, statement := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS otp_codes_live_email_purpose_unique
			ON otp_codes (email, purpose) WHERE consumed_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS otp_codes_email_purpose_created_at_index
			ON otp_codes (email, purpose, created_at)`,
		`CREATE INDEX IF NOT EXISTS otp_codes_live_purpose_index
			ON otp_codes (purpose) WHERE consumed_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS otp_codes_user_id_index ON otp_codes (user_id)`,
	} {
		if err := facades.Schema().Sql(statement); err != nil {
			return err
		}
	}

	return nil
}

// Down drops the indexes and the column again. The user is still recoverable
// from the meta of every row that had one, so nothing is lost.
func (r *M20260915000001AddUserIDAndIndexesToOtpCodesTable) Down() error {
	if !facades.Schema().HasTable("otp_codes") {
		return nil
	}

	for _, statement := range []string{
		`DROP INDEX IF EXISTS otp_codes_live_email_purpose_unique`,
		`DROP INDEX IF EXISTS otp_codes_email_purpose_created_at_index`,
		`DROP INDEX IF EXISTS otp_codes_live_purpose_index`,
		`DROP INDEX IF EXISTS otp_codes_user_id_index`,
		`ALTER TABLE otp_codes DROP COLUMN IF EXISTS user_id`,
	} {
		if err := facades.Schema().Sql(statement); err != nil {
			return err
		}
	}

	return nil
}
