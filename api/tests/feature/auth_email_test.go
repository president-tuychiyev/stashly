package feature

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// verifiedPassword satisfies the panel policy: eight characters, a letter and
// a digit.
const verifiedPassword = "Freshpass1"

func TestLoginWithEmailReturnsTheUser(t *testing.T) {
	email := uniqueEmail(t)
	makeAdmin(t, email)

	recorder := adminRequest(t, http.MethodPost, "/admin/auth/login",
		map[string]string{"email": email, "password": testUserPassword}, "")
	require.Equal(t, http.StatusOK, recorder.Code, describe(recorder))

	body := decode(t, recorder)
	user, ok := body["user"].(map[string]any)
	require.True(t, ok, describe(recorder))
	require.Equal(t, email, user["email"])
	require.Equal(t, models.UserStatusActive, user["status"])
	require.NotContains(t, user, "phone")
}

func TestLoginRefusesPendingAndBlockedAccounts(t *testing.T) {
	for status, message := range map[string]string{
		models.UserStatusPending: "account not verified",
		models.UserStatusBlocked: "account blocked",
	} {
		email := uniqueEmail(t)
		makeUserWithStatus(t, email, seededRole(t, "admin"), status)

		recorder := adminRequest(t, http.MethodPost, "/admin/auth/login",
			map[string]string{"email": email, "password": testUserPassword}, "")
		require.Equal(t, http.StatusForbidden, recorder.Code, describe(recorder))
		require.Equal(t, message, decode(t, recorder)["message"])
	}
}

func TestVerifySetsThePasswordAndActivatesTheAccount(t *testing.T) {
	email := uniqueEmail(t)
	user := makeUserWithStatus(t, email, seededRole(t, "admin"), models.UserStatusPending)

	require.NoError(t, services.NewOtpService().Issue(services.OtpIssueInput{
		Email:   email,
		Purpose: models.OtpPurposeVerify,
		Name:    user.Name,
		Meta:    map[string]any{"user_id": user.ID},
	}))

	code := otpCodeFor(t, email, models.OtpPurposeVerify)

	// A wrong code changes nothing.
	wrong := adminRequest(t, http.MethodPost, "/admin/auth/verify", map[string]string{
		"email": email, "code": "000000", "password": verifiedPassword,
		"password_confirmation": verifiedPassword,
	}, "")
	if wrong.Code != http.StatusUnprocessableEntity {
		// The generated code could be 000000; then the call above succeeded and
		// there is nothing left to verify.
		require.Equal(t, "000000", code, describe(wrong))

		return
	}

	recorder := adminRequest(t, http.MethodPost, "/admin/auth/verify", map[string]string{
		"email": email, "code": code, "password": verifiedPassword,
		"password_confirmation": verifiedPassword,
	}, "")
	require.Equal(t, http.StatusOK, recorder.Code, describe(recorder))

	body := decode(t, recorder)
	require.NotEmpty(t, body["token"])

	var reloaded models.User
	require.NoError(t, facades.Orm().Query().Where("id", user.ID).First(&reloaded))
	require.Equal(t, models.UserStatusActive, reloaded.Status)
	require.NotNil(t, reloaded.EmailVerifiedAt)

	// The new password works, and the code cannot be replayed.
	after := adminRequest(t, http.MethodPost, "/admin/auth/login",
		map[string]string{"email": email, "password": verifiedPassword}, "")
	require.Equal(t, http.StatusOK, after.Code, describe(after))

	replay := adminRequest(t, http.MethodPost, "/admin/auth/verify", map[string]string{
		"email": email, "code": code, "password": verifiedPassword,
		"password_confirmation": verifiedPassword,
	}, "")
	require.Equal(t, http.StatusUnprocessableEntity, replay.Code, describe(replay))
}

func TestVerifyRejectsAWeakPassword(t *testing.T) {
	email := uniqueEmail(t)
	makeUserWithStatus(t, email, seededRole(t, "admin"), models.UserStatusPending)

	recorder := adminRequest(t, http.MethodPost, "/admin/auth/verify", map[string]string{
		"email": email, "code": "123456", "password": "alllettersonly",
		"password_confirmation": "alllettersonly",
	}, "")
	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code, describe(recorder))
	require.Contains(t, recorder.Body.String(), "password")
}

func TestResendIsRefusedInsideTheResendGap(t *testing.T) {
	email := uniqueEmail(t)
	makeUserWithStatus(t, email, seededRole(t, "admin"), models.UserStatusPending)

	first := adminRequest(t, http.MethodPost, "/admin/auth/verify/resend",
		map[string]string{"email": email}, "")
	require.Equal(t, http.StatusNoContent, first.Code, describe(first))

	second := adminRequest(t, http.MethodPost, "/admin/auth/verify/resend",
		map[string]string{"email": email}, "")
	require.Equal(t, http.StatusTooManyRequests, second.Code, describe(second))
}

func TestResendSaysNothingAboutUnknownAddresses(t *testing.T) {
	recorder := adminRequest(t, http.MethodPost, "/admin/auth/verify/resend",
		map[string]string{"email": uniqueEmail(t)}, "")
	require.Equal(t, http.StatusNoContent, recorder.Code, describe(recorder))
}

func TestForgotAndResetChangeThePassword(t *testing.T) {
	email := uniqueEmail(t)
	makeAdmin(t, email)

	// An address nobody owns answers exactly like one that exists.
	unknown := adminRequest(t, http.MethodPost, "/admin/auth/forgot",
		map[string]string{"email": uniqueEmail(t)}, "")
	require.Equal(t, http.StatusNoContent, unknown.Code, describe(unknown))

	sent := adminRequest(t, http.MethodPost, "/admin/auth/forgot",
		map[string]string{"email": email}, "")
	require.Equal(t, http.StatusNoContent, sent.Code, describe(sent))

	code := otpCodeFor(t, email, models.OtpPurposePasswordReset)

	reset := adminRequest(t, http.MethodPost, "/admin/auth/reset", map[string]string{
		"email": email, "code": code, "password": verifiedPassword,
		"password_confirmation": verifiedPassword,
	}, "")
	require.Equal(t, http.StatusNoContent, reset.Code, describe(reset))

	after := adminRequest(t, http.MethodPost, "/admin/auth/login",
		map[string]string{"email": email, "password": verifiedPassword}, "")
	require.Equal(t, http.StatusOK, after.Code, describe(after))

	stale := adminRequest(t, http.MethodPost, "/admin/auth/login",
		map[string]string{"email": email, "password": testUserPassword}, "")
	require.Equal(t, http.StatusUnauthorized, stale.Code, describe(stale))
}

func TestOtpIsInvalidatedAfterTooManyWrongAttempts(t *testing.T) {
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

	for attempt := 1; attempt <= services.OtpMaxAttempts; attempt++ {
		_, err := otp.Verify(email, models.OtpPurposePasswordReset, wrong)
		require.ErrorIs(t, err, services.ErrOtpInvalid, "attempt %d", attempt)
	}

	// The right code is worthless once the code has been burnt.
	_, err := otp.Verify(email, models.OtpPurposePasswordReset, code)
	require.ErrorIs(t, err, services.ErrOtpInvalid)
}
