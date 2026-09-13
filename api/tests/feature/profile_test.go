package feature

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

func TestProfilePasswordChangeNeedsTheCurrentPassword(t *testing.T) {
	email := uniqueEmail(t)
	makeAdmin(t, email)
	token := login(t, email)

	wrong := adminRequest(t, http.MethodPut, "/admin/profile/password", map[string]string{
		"current_password": "not-the-password", "password": verifiedPassword,
		"password_confirmation": verifiedPassword,
	}, token)
	require.Equal(t, http.StatusUnprocessableEntity, wrong.Code, describe(wrong))
	require.Contains(t, wrong.Body.String(), "current_password")

	mismatch := adminRequest(t, http.MethodPut, "/admin/profile/password", map[string]string{
		"current_password": testUserPassword, "password": verifiedPassword,
		"password_confirmation": "Somethingelse1",
	}, token)
	require.Equal(t, http.StatusUnprocessableEntity, mismatch.Code, describe(mismatch))

	ok := adminRequest(t, http.MethodPut, "/admin/profile/password", map[string]string{
		"current_password": testUserPassword, "password": verifiedPassword,
		"password_confirmation": verifiedPassword,
	}, token)
	require.Equal(t, http.StatusNoContent, ok.Code, describe(ok))

	after := adminRequest(t, http.MethodPost, "/admin/auth/login",
		map[string]string{"email": email, "password": verifiedPassword}, "")
	require.Equal(t, http.StatusOK, after.Code, describe(after))
}

func TestProfileEmailChangeSendsTheCodeToTheNewAddress(t *testing.T) {
	email := uniqueEmail(t)
	target := uniqueEmail(t)
	user := makeAdmin(t, email)
	token := login(t, email)

	t.Cleanup(func() { forgetOtpCodes(target) })

	requested := adminRequest(t, http.MethodPost, "/admin/profile/email",
		map[string]string{"email": target, "current_password": testUserPassword}, token)
	require.Equal(t, http.StatusNoContent, requested.Code, describe(requested))

	// The code lives under the new address, never the old one.
	code := otpCodeFor(t, target, models.OtpPurposeEmailChange)

	// Nothing changed yet.
	var before models.User
	require.NoError(t, facades.Orm().Query().Where("id", user.ID).First(&before))
	require.Equal(t, email, before.Email)

	confirmed := adminRequest(t, http.MethodPost, "/admin/profile/email/confirm",
		map[string]string{"code": code}, token)
	require.Equal(t, http.StatusOK, confirmed.Code, describe(confirmed))

	var after models.User
	require.NoError(t, facades.Orm().Query().Where("id", user.ID).First(&after))
	require.Equal(t, target, after.Email)

	// The account is reachable under the new address and not the old one.
	fresh := adminRequest(t, http.MethodPost, "/admin/auth/login",
		map[string]string{"email": target, "password": testUserPassword}, "")
	require.Equal(t, http.StatusOK, fresh.Code, describe(fresh))

	stale := adminRequest(t, http.MethodPost, "/admin/auth/login",
		map[string]string{"email": email, "password": testUserPassword}, "")
	require.Equal(t, http.StatusUnauthorized, stale.Code, describe(stale))
}

func TestProfileEmailChangeRefusesATakenAddress(t *testing.T) {
	email := uniqueEmail(t)
	taken := uniqueEmail(t)
	makeAdmin(t, email)
	makeAdmin(t, taken)
	token := login(t, email)

	recorder := adminRequest(t, http.MethodPost, "/admin/profile/email",
		map[string]string{"email": taken, "current_password": testUserPassword}, token)
	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code, describe(recorder))
	require.Contains(t, recorder.Body.String(), "email")
}

func TestProfileUpdatesTheName(t *testing.T) {
	email := uniqueEmail(t)
	makeAdmin(t, email)
	token := login(t, email)

	recorder := adminRequest(t, http.MethodPut, "/admin/profile",
		map[string]string{"name": "Renamed Admin"}, token)
	require.Equal(t, http.StatusOK, recorder.Code, describe(recorder))

	data, ok := decode(t, recorder)["data"].(map[string]any)
	require.True(t, ok, describe(recorder))
	require.Equal(t, "Renamed Admin", data["name"])
	require.Equal(t, float64(0), data["clients_count"])
}
