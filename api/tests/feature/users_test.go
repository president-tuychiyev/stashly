package feature

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

func TestAdminRoleCannotReachUserManagement(t *testing.T) {
	email := uniqueEmail(t)
	makeAdmin(t, email)
	token := login(t, email)

	// Everything else on /admin is open to the admin role.
	dashboard := adminRequest(t, http.MethodGet, "/admin/dashboard", nil, token)
	require.Equal(t, http.StatusOK, dashboard.Code, describe(dashboard))

	for _, target := range []string{"/admin/users", "/admin/users/1"} {
		recorder := adminRequest(t, http.MethodGet, target, nil, token)
		require.Equal(t, http.StatusForbidden, recorder.Code, "%s: %s", target, describe(recorder))
	}

	created := adminRequest(t, http.MethodPost, "/admin/users",
		map[string]string{"name": "nope", "email": uniqueEmail(t), "role": "admin"}, token)
	require.Equal(t, http.StatusForbidden, created.Code, describe(created))
}

func TestSuperAdminCreatesAPendingUserWithACode(t *testing.T) {
	email := uniqueEmail(t)
	invited := uniqueEmail(t)
	makeSuperAdmin(t, email)
	token := login(t, email)

	recorder := adminRequest(t, http.MethodPost, "/admin/users",
		map[string]string{"name": "Invited Admin", "email": invited, "role": "admin"}, token)
	require.Equal(t, http.StatusCreated, recorder.Code, describe(recorder))

	data, ok := decode(t, recorder)["data"].(map[string]any)
	require.True(t, ok, describe(recorder))
	require.Equal(t, models.UserStatusPending, data["status"])

	var user models.User
	require.NoError(t, facades.Orm().Query().Where("email", invited).First(&user))
	t.Cleanup(func() { removeUser(&user) })
	require.Nil(t, user.Password)

	count, err := facades.Orm().Query().Model(&models.OtpCode{}).
		Where("email", invited).Where("purpose", models.OtpPurposeVerify).Count()
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	// The invited user activates itself with the emailed code.
	code := otpCodeFor(t, invited, models.OtpPurposeVerify)
	verified := adminRequest(t, http.MethodPost, "/admin/auth/verify", map[string]string{
		"email": invited, "code": code, "password": verifiedPassword,
		"password_confirmation": verifiedPassword,
	}, "")
	require.Equal(t, http.StatusOK, verified.Code, describe(verified))
}

func TestSuperAdminCannotBlockOrDeleteItself(t *testing.T) {
	email := uniqueEmail(t)
	me := makeSuperAdmin(t, email)
	token := login(t, email)

	blocked := adminRequest(t, http.MethodPut, fmt.Sprintf("/admin/users/%d", me.ID),
		map[string]string{"status": models.UserStatusBlocked}, token)
	require.Equal(t, http.StatusUnprocessableEntity, blocked.Code, describe(blocked))

	demoted := adminRequest(t, http.MethodPut, fmt.Sprintf("/admin/users/%d", me.ID),
		map[string]string{"role": "admin"}, token)
	require.Equal(t, http.StatusUnprocessableEntity, demoted.Code, describe(demoted))

	deleted := adminRequest(t, http.MethodDelete, fmt.Sprintf("/admin/users/%d", me.ID), nil, token)
	require.Equal(t, http.StatusUnprocessableEntity, deleted.Code, describe(deleted))
}

func TestDeletingAnOwnerRequiresReassigningItsClients(t *testing.T) {
	email := uniqueEmail(t)
	makeSuperAdmin(t, email)
	token := login(t, email)

	owner := makeAdmin(t, uniqueEmail(t))
	successor := makeAdmin(t, uniqueEmail(t))

	client := makeClient(t, fmt.Sprintf("owned-%d", time.Now().UnixNano()), func(client *models.Client) {
		client.OwnerID = &owner.ID
	})

	// Without a successor the delete is refused, and the owner is still there.
	refused := adminRequest(t, http.MethodDelete, fmt.Sprintf("/admin/users/%d", owner.ID), nil, token)
	require.Equal(t, http.StatusUnprocessableEntity, refused.Code, describe(refused))
	require.Contains(t, refused.Body.String(), "reassign_to")

	accepted := adminRequest(t, http.MethodDelete,
		fmt.Sprintf("/admin/users/%d?reassign_to=%d", owner.ID, successor.ID), nil, token)
	require.Equal(t, http.StatusNoContent, accepted.Code, describe(accepted))

	var moved models.Client
	require.NoError(t, facades.Orm().Query().Where("id", client.ID).First(&moved))
	require.NotNil(t, moved.OwnerID)
	require.Equal(t, successor.ID, *moved.OwnerID)

	gone, err := facades.Orm().Query().Model(&models.User{}).Where("id", owner.ID).Exists()
	require.NoError(t, err)
	require.False(t, gone)
}

func TestUserListingFiltersAndCountsClients(t *testing.T) {
	email := uniqueEmail(t)
	makeSuperAdmin(t, email)
	token := login(t, email)

	owner := makeAdmin(t, uniqueEmail(t))
	makeClient(t, fmt.Sprintf("counted-%d", time.Now().UnixNano()), func(client *models.Client) {
		client.OwnerID = &owner.ID
	})

	recorder := adminRequest(t, http.MethodGet,
		"/admin/users?search="+services.NormalizeEmail(owner.Email), nil, token)
	require.Equal(t, http.StatusOK, recorder.Code, describe(recorder))

	body := decode(t, recorder)
	rows, ok := body["data"].([]any)
	require.True(t, ok, describe(recorder))
	require.Len(t, rows, 1, describe(recorder))

	row, ok := rows[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, owner.Email, row["email"])
	require.Equal(t, float64(1), row["clients_count"])
}
