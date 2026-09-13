package seeders

import (
	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

type RoleSeeder struct{}

func (r *RoleSeeder) Signature() string {
	return "RoleSeeder"
}

func (r *RoleSeeder) Run() error {
	roles := []models.Role{
		{Name: "Super Admin", Slug: "super_admin", Permissions: []string{}, IsActive: true},
		{Name: "Admin", Slug: "admin", Permissions: []string{}, IsActive: true},
	}

	for _, role := range roles {
		if err := facades.Orm().Query().UpdateOrCreate(&models.Role{}, models.Role{Slug: role.Slug}, role); err != nil {
			return err
		}
	}

	return nil
}
