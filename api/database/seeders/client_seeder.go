package seeders

import (
	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

type ClientSeeder struct{}

func (r *ClientSeeder) Signature() string {
	return "ClientSeeder"
}

func (r *ClientSeeder) Run() error {
	password, err := facades.Hash().Make("demo12345")
	if err != nil {
		return err
	}

	// The demo client belongs to the seeded super admin, so a fresh database
	// never carries a client without an owner.
	var owner models.User
	if err := facades.Orm().Query().Where("email", SeedAdminEmail).FirstOrFail(&owner); err != nil {
		return err
	}

	name := "Demo client"
	username := "demo"
	quota := int64(1073741824)

	client := models.Client{
		Name:         &name,
		Username:     &username,
		Password:     password,
		Status:       "active",
		QuotaBytes:   &quota,
		AllowedMimes: []string{},
		OwnerID:      &owner.ID,
		CreatorID:    &owner.ID,
		UpdaterID:    &owner.ID,
	}

	return facades.Orm().Query().UpdateOrCreate(&models.Client{}, models.Client{Username: &username}, client)
}
