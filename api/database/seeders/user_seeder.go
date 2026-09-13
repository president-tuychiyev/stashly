package seeders

import (
	"errors"

	"github.com/goravel/framework/support/carbon"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// SeedAdminEmail is the address of the super admin the seeder guarantees.
const SeedAdminEmail = "stashly@example.com"

// defaultSeedAdminPassword is used when SEED_ADMIN_PASSWORD is not set. It is
// meant for a fresh development database and has to be changed anywhere else.
const defaultSeedAdminPassword = "ChangeMe123!"

type UserSeeder struct{}

func (r *UserSeeder) Signature() string {
	return "UserSeeder"
}

func (r *UserSeeder) Run() error {
	var role models.Role
	if err := facades.Orm().Query().Where("slug", "super_admin").FirstOrFail(&role); err != nil {
		return err
	}

	// FirstOrCreate, not UpdateOrCreate: re-running the seeders on a live
	// database must never touch an account that already exists. Overwriting
	// the password would hand the panel back to whoever knows the seed
	// password, and overwriting status or role would silently un-block or
	// re-promote an account somebody deliberately changed.
	var existing models.User
	if err := facades.Orm().Query().Model(&models.User{}).
		Where("email", SeedAdminEmail).First(&existing); err != nil {
		return err
	}
	if existing.ID != 0 {
		return r.dropLegacyUsers()
	}

	plain, err := seedAdminPassword()
	if err != nil {
		return err
	}

	password, err := facades.Hash().Make(plain)
	if err != nil {
		return err
	}

	now := carbon.NewDateTime(carbon.Now())
	user := &models.User{
		Name:                 "Stashly Admin",
		Email:                SeedAdminEmail,
		Password:             &password,
		Status:               models.UserStatusActive,
		EmailVerifiedAt:      now,
		CredentialsChangedAt: now,
		RoleID:               &role.ID,
	}
	if err := facades.Orm().Query().Create(user); err != nil {
		return err
	}

	return r.dropLegacyUsers()
}

// seedAdminPassword returns the password the super admin is created with.
//
// The built in default is a development convenience and is published in this
// repository, so it is only allowed where a leak costs nothing: APP_ENV local
// or testing. Anywhere else the seeder refuses to run rather than create a
// super admin whose password everybody already knows.
func seedAdminPassword() (string, error) {
	plain := facades.Config().EnvString("SEED_ADMIN_PASSWORD", "")

	if (plain == "" || plain == defaultSeedAdminPassword) && !services.AppIsLocalOrTesting() {
		return "", errors.New(
			"refusing to seed the super admin with the built in default password outside a local or testing " +
				"environment: set SEED_ADMIN_PASSWORD to a password of your own and run the seeder again")
	}

	if plain == "" {
		return defaultSeedAdminPassword, nil
	}

	return plain, nil
}

// dropLegacyUsers removes the phone based account the scaffold shipped. The
// email migration gave it a synthetic "@legacy.local" address, which is what
// identifies it here; whatever it still owns is handed to the seeded super
// admin first so no client is left without an owner.
func (r *UserSeeder) dropLegacyUsers() error {
	var seeded models.User
	if err := facades.Orm().Query().Where("email", SeedAdminEmail).FirstOrFail(&seeded); err != nil {
		return err
	}

	var legacy []models.User
	if err := facades.Orm().Query().Model(&models.User{}).
		Where("email LIKE ?", "%@legacy.local").Get(&legacy); err != nil {
		return err
	}

	for index := range legacy {
		if _, err := facades.Orm().Query().Model(&models.Client{}).
			Where("owner_id", legacy[index].ID).Update("owner_id", seeded.ID); err != nil {
			return err
		}
		if _, err := facades.Orm().Query().ForceDelete(&legacy[index]); err != nil {
			return err
		}
	}

	return nil
}
