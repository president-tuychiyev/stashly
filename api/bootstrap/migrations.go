package bootstrap

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/president-tuychiyev/stashly/api/database/migrations"
)

func Migrations() []schema.Migration {
	return []schema.Migration{
		&migrations.M20210101000001CreateJobsTable{},
		&migrations.M20260729063813CreateRolesTable{},
		&migrations.M20260729063801CreateUsersTable{},
		&migrations.M20260729063825CreateClientsTable{},
		&migrations.M20260729063836CreateDevicesTable{},
		&migrations.M20260912000001AddStorageFieldsToClientsTable{},
		&migrations.M20260912000002CreateFilesTable{},
		&migrations.M20260912000003CreateArchivesTable{},
		&migrations.M20260912000004CreateAuditLogsTable{},
		&migrations.M20260913000001PartialUniqueUsernameOnClientsTable{},
		&migrations.M20260914000001AlterUsersForEmailAuth{},
		&migrations.M20260914000002AddOwnerIDToClientsTable{},
		&migrations.M20260914000003CreateOtpCodesTable{},
		&migrations.M20260914000004AddClientIDToAuditLogsTable{},
		&migrations.M20260915000001AddUserIDAndIndexesToOtpCodesTable{},
		&migrations.M20260915000002AddCredentialsChangedAtToUsersTable{},
	}
}
