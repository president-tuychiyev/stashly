package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

type M20210101000001CreateJobsTable struct{}

// Signature The unique signature for the migration.
//
// The string deliberately reads 20210101000002 while the file and the type say
// 20210101000001. The signature is what the migrations table stores, so
// correcting it here would make every existing database believe the migration
// has never run and try to create the jobs tables again. The mismatch is
// cosmetic; leave it alone.
func (r *M20210101000001CreateJobsTable) Signature() string {
	return "20210101000002_create_jobs_table"
}

// Up Run the migrations.
func (r *M20210101000001CreateJobsTable) Up() error {
	if !facades.Schema().HasTable("jobs") {
		if err := facades.Schema().Create("jobs", func(table schema.Blueprint) {
			table.ID()
			table.String("queue")
			table.LongText("payload")
			table.UnsignedTinyInteger("attempts").Default(0)
			table.DateTimeTz("reserved_at").Nullable()
			table.DateTimeTz("available_at")
			table.DateTimeTz("created_at").UseCurrent()
			table.Index("queue")
		}); err != nil {
			return err
		}
	}

	if !facades.Schema().HasTable("failed_jobs") {
		if err := facades.Schema().Create("failed_jobs", func(table schema.Blueprint) {
			table.ID()
			table.String("uuid")
			table.Text("connection")
			table.Text("queue")
			table.LongText("payload")
			table.LongText("exception")
			table.DateTimeTz("failed_at").UseCurrent()
			table.Unique("uuid")
		}); err != nil {
			return err
		}
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20210101000001CreateJobsTable) Down() error {
	if err := facades.Schema().DropIfExists("jobs"); err != nil {
		return err
	}

	if err := facades.Schema().DropIfExists("failed_jobs"); err != nil {
		return err
	}

	return nil
}
