package feature

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
	"github.com/president-tuychiyev/stashly/api/database/seeders"
)

// TestOtpAttemptsAreCountedAtomically fires more wrong guesses at one code
// than it is allowed, all at once.
//
// The counter used to be read, incremented in Go and written back, so ten
// parallel guesses could each read "0" and the code would survive all of them.
// The increment is done by the database now, so the code is burnt whatever the
// interleaving is.
func TestOtpAttemptsAreCountedAtomically(t *testing.T) {
	email := uniqueEmail(t)
	otp := services.NewOtpService()

	require.NoError(t, otp.Issue(services.OtpIssueInput{
		Email:   email,
		Purpose: models.OtpPurposePasswordReset,
	}))
	t.Cleanup(func() { forgetOtpCodes(email) })

	code := otpCodeFor(t, email, models.OtpPurposePasswordReset)
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}

	var group sync.WaitGroup
	for guess := 0; guess < 10; guess++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, _ = otp.Verify(email, models.OtpPurposePasswordReset, wrong)
		}()
	}
	group.Wait()

	// The row is burnt: consumed, and with at least the maximum number of
	// attempts recorded against it.
	var record models.OtpCode
	require.NoError(t, facades.Orm().Query().Model(&models.OtpCode{}).
		Where("email", services.NormalizeEmail(email)).
		Where("purpose", models.OtpPurposePasswordReset).
		OrderByDesc("id").First(&record))
	require.NotZero(t, record.ID)
	require.NotNil(t, record.ConsumedAt, "the code survived ten parallel wrong guesses")
	require.GreaterOrEqual(t, int(record.Attempts), services.OtpMaxAttempts)

	// And the code everybody was guessing at is worthless.
	_, err := otp.Verify(email, models.OtpPurposePasswordReset, code)
	require.ErrorIs(t, err, services.ErrOtpInvalid)
}

// TestOtpConsumeIsConditional presents the same correct code twice. The second
// presentation has to fail: consuming is conditional on the row still being
// unconsumed, so a replay cannot slip through between the check and the write.
func TestOtpConsumeIsConditional(t *testing.T) {
	email := uniqueEmail(t)
	otp := services.NewOtpService()

	require.NoError(t, otp.Issue(services.OtpIssueInput{
		Email:   email,
		Purpose: models.OtpPurposePasswordReset,
	}))
	t.Cleanup(func() { forgetOtpCodes(email) })

	code := otpCodeFor(t, email, models.OtpPurposePasswordReset)

	_, err := otp.Verify(email, models.OtpPurposePasswordReset, code)
	require.NoError(t, err)

	_, err = otp.Verify(email, models.OtpPurposePasswordReset, code)
	require.ErrorIs(t, err, services.ErrOtpInvalid)
}

// TestEmailChangeCodeOfAnotherUserIsRejected checks that a code is bound to the
// account it was issued for.
//
// The code is stored under the NEW address, which the confirming user never
// sends, so the endpoint has to find the pending change by user. Somebody who
// gets hold of a code meant for another account must not be able to redirect
// their own address with it.
func TestEmailChangeCodeOfAnotherUserIsRejected(t *testing.T) {
	owner := uniqueEmail(t)
	intruder := uniqueEmail(t)
	target := uniqueEmail(t)
	ownerUser := makeAdmin(t, owner)
	makeAdmin(t, intruder)

	t.Cleanup(func() { forgetOtpCodes(target) })

	ownerToken := login(t, owner)
	intruderToken := login(t, intruder)

	requested := adminRequest(t, http.MethodPost, "/admin/profile/email",
		map[string]string{"email": target, "current_password": testUserPassword}, ownerToken)
	require.Equal(t, http.StatusNoContent, requested.Code, describe(requested))

	code := otpCodeFor(t, target, models.OtpPurposeEmailChange)

	// The code belongs to the owner, so the other account cannot spend it.
	stolen := adminRequest(t, http.MethodPost, "/admin/profile/email/confirm",
		map[string]string{"code": code}, intruderToken)
	require.Equal(t, http.StatusUnprocessableEntity, stolen.Code, describe(stolen))
	require.Contains(t, stolen.Body.String(), services.ErrOtpInvalid.Error())

	// Nothing was consumed by the failed attempt, so the owner still can.
	confirmed := adminRequest(t, http.MethodPost, "/admin/profile/email/confirm",
		map[string]string{"code": code}, ownerToken)
	require.Equal(t, http.StatusOK, confirmed.Code, describe(confirmed))

	var after models.User
	require.NoError(t, facades.Orm().Query().Where("id", ownerUser.ID).First(&after))
	require.Equal(t, target, after.Email)
}

// TestUnknownAddressIsRateLimitedLikeAKnownOne closes the enumeration oracle.
//
// The resend gap used to live in the otp_codes table only, which exists for
// addresses that belong to somebody: a known address answered 429 on the
// second call while an unknown one kept answering 204 forever, and the
// difference told a caller who has an account here. The gap is claimed before
// the user is looked up now, so both answer the same.
func TestUnknownAddressIsRateLimitedLikeAKnownOne(t *testing.T) {
	unknown := uniqueEmail(t)

	first := adminRequest(t, http.MethodPost, "/admin/auth/forgot",
		map[string]string{"email": unknown}, "")
	require.Equal(t, http.StatusNoContent, first.Code, describe(first))

	second := adminRequest(t, http.MethodPost, "/admin/auth/forgot",
		map[string]string{"email": unknown}, "")
	require.Equal(t, http.StatusTooManyRequests, second.Code, describe(second))
	require.Equal(t, services.ErrOtpResendTooSoon.Error(), decode(t, second)["message"])
}

// TestRefreshRejectsABlockedUser makes sure a session cannot outlive the
// account. Refresh only looked at the claims, so a blocked user could keep
// minting fresh tokens for the whole refresh window.
func TestRefreshRejectsABlockedUser(t *testing.T) {
	email := uniqueEmail(t)
	user := makeAdmin(t, email)
	token := login(t, email)

	_, err := facades.Orm().Query().Model(&models.User{}).
		Where("id", user.ID).Update("status", models.UserStatusBlocked)
	require.NoError(t, err)

	recorder := adminRequest(t, http.MethodPost, "/admin/auth/refresh", nil, token)
	require.Equal(t, http.StatusForbidden, recorder.Code, describe(recorder))
}

// TestTokenIssuedBeforeAPasswordChangeIsRejected checks that changing a
// password really ends the other sessions.
func TestTokenIssuedBeforeAPasswordChangeIsRejected(t *testing.T) {
	email := uniqueEmail(t)
	makeAdmin(t, email)
	token := login(t, email)

	// The "iat" claim only has second resolution and credentials_changed_at is
	// compared against it truncated to the second, so a token minted in the
	// same second as the change is deliberately still accepted. Step over the
	// boundary to test the case that matters.
	time.Sleep(1100 * time.Millisecond)

	changed := adminRequest(t, http.MethodPut, "/admin/profile/password", map[string]string{
		"current_password":      testUserPassword,
		"password":              verifiedPassword,
		"password_confirmation": verifiedPassword,
	}, token)
	require.Equal(t, http.StatusNoContent, changed.Code, describe(changed))

	stale := adminRequest(t, http.MethodGet, "/admin/profile", nil, token)
	require.Equal(t, http.StatusUnauthorized, stale.Code, describe(stale))

	// A token minted after the change works again.
	fresh := adminRequest(t, http.MethodPost, "/admin/auth/login",
		map[string]string{"email": email, "password": verifiedPassword}, "")
	require.Equal(t, http.StatusOK, fresh.Code, describe(fresh))
	refreshed := "Bearer " + decode(t, fresh)["token"].(string)

	after := adminRequest(t, http.MethodGet, "/admin/profile", nil, refreshed)
	require.Equal(t, http.StatusOK, after.Code, describe(after))
}

// TestTheLastActiveSuperAdminIsProtected blocks and deletes the only account
// left that can manage users, and expects both to be refused.
//
// The caller is a custom role holding the user management permissions rather
// than a super admin: a super admin doing this would always leave itself
// behind, so it could never reach the case.
func TestTheLastActiveSuperAdminIsProtected(t *testing.T) {
	only := onlyActiveSuperAdmin(t)

	manager := uniqueEmail(t)
	makeUser(t, manager, makeRole(t, "user_manager", []string{
		"admin.users.index", "admin.users.show", "admin.users.update", "admin.users.destroy",
	}, true))
	token := login(t, manager)

	blocked := adminRequest(t, http.MethodPut, "/admin/users/"+itoa(only.ID),
		map[string]string{"status": models.UserStatusBlocked}, token)
	require.Equal(t, http.StatusUnprocessableEntity, blocked.Code, describe(blocked))
	require.Equal(t, "at least one active super administrator is required",
		decode(t, blocked)["message"])

	demoted := adminRequest(t, http.MethodPut, "/admin/users/"+itoa(only.ID),
		map[string]string{"role": "admin"}, token)
	require.Equal(t, http.StatusUnprocessableEntity, demoted.Code, describe(demoted))

	destroyed := adminRequest(t, http.MethodDelete, "/admin/users/"+itoa(only.ID), nil, token)
	require.Equal(t, http.StatusUnprocessableEntity, destroyed.Code, describe(destroyed))

	// With a second super admin around, the same calls go through.
	makeSuperAdmin(t, uniqueEmail(t))

	allowed := adminRequest(t, http.MethodPut, "/admin/users/"+itoa(only.ID),
		map[string]string{"status": models.UserStatusBlocked}, token)
	require.Equal(t, http.StatusOK, allowed.Code, describe(allowed))
}

// TestSeederDoesNotOverwriteAnExistingPassword re-runs the user seeder against
// a database that already has the super admin.
//
// The seeder used to UpdateOrCreate, so every deployment that re-ran the
// seeders handed the panel back to whoever knows the published seed password
// and un-blocked the account while it was at it.
func TestSeederDoesNotOverwriteAnExistingPassword(t *testing.T) {
	seededRole(t, "super_admin")
	seeder := &seeders.UserSeeder{}

	var before models.User
	require.NoError(t, facades.Orm().Query().Model(&models.User{}).
		Where("email", seeders.SeedAdminEmail).First(&before))
	if before.ID == 0 {
		require.NoError(t, seeder.Run())
		require.NoError(t, facades.Orm().Query().Model(&models.User{}).
			Where("email", seeders.SeedAdminEmail).First(&before))
		t.Cleanup(func() {
			_, _ = facades.Orm().Query().Model(&models.User{}).Where("id", before.ID).ForceDelete(&models.User{})
		})
	}

	original := before.Password
	originalStatus := before.Status
	t.Cleanup(func() {
		_, _ = facades.Orm().Query().Model(&models.User{}).Where("id", before.ID).
			Update(map[string]any{"password": original, "status": originalStatus})
	})

	// Somebody changed the password and blocked the account by hand.
	changed, err := facades.Hash().Make("Rotated-secret-9")
	require.NoError(t, err)
	_, err = facades.Orm().Query().Model(&models.User{}).Where("id", before.ID).
		Update(map[string]any{"password": changed, "status": models.UserStatusBlocked})
	require.NoError(t, err)

	require.NoError(t, seeder.Run())

	var after models.User
	require.NoError(t, facades.Orm().Query().Model(&models.User{}).
		Where("email", seeders.SeedAdminEmail).First(&after))
	require.NotNil(t, after.Password)
	require.Equal(t, changed, *after.Password, "the seeder reset the super admin password")
	require.Equal(t, models.UserStatusBlocked, after.Status, "the seeder un-blocked the super admin")
}

// onlyActiveSuperAdmin leaves exactly one active super admin in the database
// and returns it. Everybody else holding the role is parked as blocked until
// the test ends.
func onlyActiveSuperAdmin(t *testing.T) *models.User {
	t.Helper()

	role := seededRole(t, "super_admin")

	var existing []models.User
	require.NoError(t, facades.Orm().Query().Model(&models.User{}).
		Where("role_id", role.ID).Where("status", models.UserStatusActive).Get(&existing))

	for index := range existing {
		id := existing[index].ID
		status := existing[index].Status
		_, err := facades.Orm().Query().Model(&models.User{}).
			Where("id", id).Update("status", models.UserStatusBlocked)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = facades.Orm().Query().Model(&models.User{}).Where("id", id).Update("status", status)
		})
	}

	return makeSuperAdmin(t, uniqueEmail(t))
}
