package services

import (
	"errors"
	"unicode"

	"github.com/goravel/framework/support/carbon"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

// MinPasswordLength is the shortest password the panel accepts.
const MinPasswordLength = 8

// ErrPasswordMismatch is returned when the confirmation does not match.
var ErrPasswordMismatch = errors.New("the password confirmation does not match")

// ErrPasswordWeak is returned when a password breaks the policy.
var ErrPasswordWeak = errors.New("the password must be at least 8 characters long and contain a letter and a digit")

// ValidatePassword enforces the one password policy of the whole panel: at
// least eight characters, at least one letter and at least one digit, and a
// matching confirmation.
func ValidatePassword(password, confirmation string) error {
	if len([]rune(password)) < MinPasswordLength {
		return ErrPasswordWeak
	}

	hasLetter, hasDigit := false, false
	for _, char := range password {
		switch {
		case unicode.IsLetter(char):
			hasLetter = true
		case unicode.IsDigit(char):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrPasswordWeak
	}

	if password != confirmation {
		return ErrPasswordMismatch
	}

	return nil
}

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

// FindByEmail loads a live user by address. It returns nil when there is none,
// which the public endpoints answer exactly like a success.
func (r *UserService) FindByEmail(email string) *models.User {
	var user models.User
	if err := facades.Orm().Query().Model(&models.User{}).
		Where("email", NormalizeEmail(email)).First(&user); err != nil || user.ID == 0 {
		return nil
	}

	return &user
}

// EmailTaken reports whether an address already belongs to a user other than
// the excluded one.
func (r *UserService) EmailTaken(email string, exclude uint) (bool, error) {
	query := facades.Orm().Query().Model(&models.User{}).Where("email", NormalizeEmail(email))
	if exclude > 0 {
		query = query.Where("id != ?", exclude)
	}

	return query.Exists()
}

// SetPassword hashes and stores a new password. Every token issued before now
// stops working, so a password change really ends the other sessions.
func (r *UserService) SetPassword(user *models.User, plain string) error {
	hashed, err := facades.Hash().Make(plain)
	if err != nil {
		return err
	}

	user.Password = &hashed
	user.CredentialsChangedAt = carbon.NewDateTime(carbon.Now())

	return facades.Orm().Query().Save(user)
}

// TouchCredentials stamps the account so that every token issued before now is
// refused. Call it from wherever a credential changes without going through
// SetPassword: an email change, or a block.
//
// The model is updated in place as well, so a caller that saves the row
// afterwards does not write the old value back.
func (r *UserService) TouchCredentials(user *models.User) {
	if user == nil {
		return
	}

	user.CredentialsChangedAt = carbon.NewDateTime(carbon.Now())
}

// ActiveSuperAdminsExcept counts the active super admins other than the given
// user. It is what stops the last one from being blocked, demoted or deleted,
// which would leave the panel with nobody who can manage accounts.
func (r *UserService) ActiveSuperAdminsExcept(exclude uint) (int64, error) {
	role, err := r.RoleBySlug(SuperAdminSlug)
	if err != nil {
		return 0, err
	}

	query := facades.Orm().Query().Model(&models.User{}).
		Where("role_id", role.ID).
		Where("status", models.UserStatusActive)
	if exclude > 0 {
		query = query.Where("id != ?", exclude)
	}

	return query.Count()
}

// IsActiveSuperAdmin reports whether a user currently holds the super admin
// role and may sign in.
func (r *UserService) IsActiveSuperAdmin(user *models.User) bool {
	if user == nil || user.RoleID == nil || user.Status != models.UserStatusActive {
		return false
	}

	role, err := r.RoleBySlug(SuperAdminSlug)
	if err != nil {
		return false
	}

	return *user.RoleID == role.ID
}

// TouchLogin records a successful sign in.
func (r *UserService) TouchLogin(user *models.User) {
	now := carbon.NewDateTime(carbon.Now())
	user.LastLoginAt = now

	if _, err := facades.Orm().Query().Model(&models.User{}).
		Where("id", user.ID).Update("last_login_at", now.StdTime()); err != nil {
		facades.Log().Warning("failed to record a login timestamp: " + err.Error())
	}
}

// ClientsCount is how many clients a user owns, shown on the user resource.
func (r *UserService) ClientsCount(userID uint) int64 {
	count, err := facades.Orm().Query().Model(&models.Client{}).Where("owner_id", userID).Count()
	if err != nil {
		return 0
	}

	return count
}

// LoadRole fills in the role relation when it is missing.
func (r *UserService) LoadRole(user *models.User) {
	if user == nil || user.Role != nil || user.RoleID == nil {
		return
	}

	var role models.Role
	if err := facades.Orm().Query().Where("id", *user.RoleID).First(&role); err == nil && role.ID != 0 {
		user.Role = &role
	}
}

// RoleBySlug loads one of the seeded roles.
func (r *UserService) RoleBySlug(slug string) (*models.Role, error) {
	var role models.Role
	if err := facades.Orm().Query().Where("slug", slug).First(&role); err != nil || role.ID == 0 {
		return nil, errors.New("role " + slug + " is missing, run the seeders")
	}

	return &role, nil
}
