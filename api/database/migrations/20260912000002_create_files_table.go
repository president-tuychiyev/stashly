package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20260912000002CreateFilesTable struct{}

// Signature The unique signature for the migration.
func (r *M20260912000002CreateFilesTable) Signature() string {
	return "20260912000002_create_files_table"
}

// Up Run the migrations.
func (r *M20260912000002CreateFilesTable) Up() error {
	if facades.Schema().HasTable("files") {
		return nil
	}

	return facades.Schema().Create("files", func(table schema.Blueprint) {
		table.ID()
		table.UnsignedBigInteger("client_id")
		table.Foreign("client_id").References("id").On("clients").CascadeOnDelete()
		table.String("folder", 255).Default("")
		table.String("name", 255)
		table.Unique("name")
		table.String("original_name", 500)
		table.String("path", 700)
		table.Unique("path")
		table.String("mime", 255)
		table.String("extension", 20)
		table.BigInteger("size")
		table.Char("sha256", 64)
		table.Index("sha256")
		table.Enum("visibility", []any{"public", "private"}).Default("public")
		table.TimestampsTz()
		table.SoftDeletesTz()
		table.Index("client_id", "folder")
	})
}

// Down Reverse the migrations.
func (r *M20260912000002CreateFilesTable) Down() error {
	return facades.Schema().DropIfExists("files")
}
