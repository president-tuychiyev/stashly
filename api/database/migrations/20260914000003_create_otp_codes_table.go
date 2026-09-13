package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260914000003CreateOtpCodesTable struct{}

// Signature The unique signature for the migration.
func (r *M20260914000003CreateOtpCodesTable) Signature() string {
	return "20260914000003_create_otp_codes_table"
}

// Up creates the one time code store used by the invite, password reset and
// email change flows. Only the HMAC of a code is stored, never the code.
func (r *M20260914000003CreateOtpCodesTable) Up() error {
	if facades.Schema().HasTable("otp_codes") {
		return nil
	}

	return facades.Schema().Create("otp_codes", func(table schema.Blueprint) {
		table.ID()
		table.String("email", 255)
		table.Enum("purpose", []any{"verify", "email_change", "password_reset"})
		table.Index("email", "purpose")
		table.String("code_hash", 128)
		table.TimestampTz("expires_at")
		table.SmallInteger("attempts").Default(0)
		table.TimestampTz("consumed_at").Nullable()
		table.Jsonb("meta").Default("{}")
		table.TimestampTz("created_at").UseCurrent()
	})
}

// Down Reverse the migrations.
func (r *M20260914000003CreateOtpCodesTable) Down() error {
	return facades.Schema().DropIfExists("otp_codes")
}
