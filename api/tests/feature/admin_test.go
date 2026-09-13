package feature

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goravel/framework/support/carbon"
	"github.com/stretchr/testify/require"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// testUserPassword is the plain password every test admin is created with.
const testUserPassword = "Admin-password-1"

// emailCounter makes uniqueEmail unique even for two calls inside the same
// clock tick: UnixNano only advances in microsecond steps on some platforms.
var emailCounter atomic.Uint64

// uniqueEmail keeps every test on its own login rate limit bucket, which is
// keyed by IP and email together.
func uniqueEmail(t *testing.T) string {
	t.Helper()

	return fmt.Sprintf("user-%d-%d@example.test", time.Now().UnixNano(), emailCounter.Add(1))
}

// makeRole creates a throw away role and removes it when the test ends.
func makeRole(t *testing.T, slug string, permissions []string, active bool) *models.Role {
	t.Helper()

	slug = fmt.Sprintf("%s-%d", slug, time.Now().UnixNano())
	role := &models.Role{
		Name:        slug,
		Slug:        slug,
		Permissions: permissions,
		IsActive:    active,
	}
	require.NoError(t, facades.Orm().Query().Create(role))

	t.Cleanup(func() {
		_, _ = facades.Orm().Query().ForceDelete(role)
		services.ForgetRole(role.ID)
	})

	return role
}

// makeUser creates a throw away, already active admin user and removes it when
// the test ends.
func makeUser(t *testing.T, email string, role *models.Role) *models.User {
	t.Helper()

	return makeUserWithStatus(t, email, role, models.UserStatusActive)
}

// makeUserWithStatus is makeUser with an explicit lifecycle status.
func makeUserWithStatus(t *testing.T, email string, role *models.Role, status string) *models.User {
	t.Helper()

	hashed, err := facades.Hash().Make(testUserPassword)
	require.NoError(t, err)

	user := &models.User{
		Name:     "test admin",
		Email:    services.NormalizeEmail(email),
		Password: &hashed,
		Status:   status,
	}
	if status == models.UserStatusActive {
		user.EmailVerifiedAt = carbon.NewDateTime(carbon.Now())
	}
	if role != nil {
		user.RoleID = &role.ID
	}
	require.NoError(t, facades.Orm().Query().Create(user))

	t.Cleanup(func() {
		removeUser(user)
	})

	return user
}

// removeUser purges a user and everything that points at it.
func removeUser(user *models.User) {
	_, _ = facades.Orm().Query().Model(&models.AuditLog{}).
		Where("actor_type", "user").Where("actor_id", user.ID).Delete()
	_, _ = facades.Orm().Query().Model(&models.OtpCode{}).Where("email", user.Email).Delete()
	_, _ = facades.Orm().Query().Model(&models.Client{}).Where("owner_id", user.ID).Update("owner_id", nil)
	_, _ = facades.Orm().Query().ForceDelete(user)
}

// makeSuperAdmin creates a user carrying the seeded super_admin role.
func makeSuperAdmin(t *testing.T, email string) *models.User {
	t.Helper()

	return makeUser(t, email, seededRole(t, "super_admin"))
}

// makeAdmin creates a user carrying the seeded admin role.
func makeAdmin(t *testing.T, email string) *models.User {
	t.Helper()

	return makeUser(t, email, seededRole(t, "admin"))
}

// seededRole loads one of the two roles the seeder ships, creating it when the
// test database has not been seeded yet.
func seededRole(t *testing.T, slug string) *models.Role {
	t.Helper()

	var role models.Role
	if err := facades.Orm().Query().Where("slug", slug).First(&role); err == nil && role.ID != 0 {
		return &role
	}

	role = models.Role{Name: slug, Slug: slug, Permissions: []string{}, IsActive: true}
	require.NoError(t, facades.Orm().Query().Create(&role))

	return &role
}

// adminRequest performs a JSON request against the running route stack.
func adminRequest(t *testing.T, method, target string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	request := httptest.NewRequest(method, target, reader)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", token)
	}

	recorder := httptest.NewRecorder()
	facades.Route().ServeHTTP(recorder, request)

	return recorder
}

// decode reads the JSON body of a recorder.
func decode(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	out := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &out), describe(recorder))

	return out
}

// login exchanges the test credentials for a bearer token.
func login(t *testing.T, email string) string {
	t.Helper()

	recorder := adminRequest(t, http.MethodPost, "/admin/auth/login",
		map[string]string{"email": email, "password": testUserPassword}, "")
	require.Equal(t, http.StatusOK, recorder.Code, describe(recorder))

	body := decode(t, recorder)
	token, ok := body["token"].(string)
	require.True(t, ok, describe(recorder))

	return "Bearer " + token
}

func TestAdminLoginSucceeds(t *testing.T) {
	email := uniqueEmail(t)
	role := makeRole(t, "login_ok", []string{"admin.auth.me"}, true)
	makeUser(t, email, role)

	token := login(t, email)

	recorder := adminRequest(t, http.MethodGet, "/admin/auth/me", nil, token)
	require.Equal(t, http.StatusOK, recorder.Code, describe(recorder))
}

func TestAdminLoginRejectsWrongPassword(t *testing.T) {
	email := uniqueEmail(t)
	makeUser(t, email, nil)

	recorder := adminRequest(t, http.MethodPost, "/admin/auth/login",
		map[string]string{"email": email, "password": "not-the-password"}, "")
	require.Equal(t, http.StatusUnauthorized, recorder.Code, describe(recorder))
}

func TestAdminLoginIsRateLimited(t *testing.T) {
	email := uniqueEmail(t)
	makeUser(t, email, nil)

	body := map[string]string{"email": email, "password": "not-the-password"}

	// The limiter allows 10 attempts per minute for one IP and email pair.
	for attempt := 1; attempt <= 10; attempt++ {
		recorder := adminRequest(t, http.MethodPost, "/admin/auth/login", body, "")
		require.Equal(t, http.StatusUnauthorized, recorder.Code,
			"attempt %d: %s", attempt, describe(recorder))
	}

	recorder := adminRequest(t, http.MethodPost, "/admin/auth/login", body, "")
	require.Equal(t, http.StatusTooManyRequests, recorder.Code, describe(recorder))
	require.Equal(t, "too many attempts", decode(t, recorder)["message"])
}

func TestJwtAuthRejectsMissingAndBadTokens(t *testing.T) {
	for name, token := range map[string]string{
		"no token":      "",
		"garbage token": "Bearer not-a-jwt",
		"wrong secret":  "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJrZXkiOiIxIiwic3ViIjoidXNlciJ9.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	} {
		recorder := adminRequest(t, http.MethodGet, "/admin/dashboard", nil, token)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, "%s: %s", name, describe(recorder))
	}
}

func TestAdminRefreshAcceptsExpiredTokenAndRejectsGarbage(t *testing.T) {
	email := uniqueEmail(t)
	role := makeRole(t, "refresh_role", []string{"admin.dashboard.index"}, true)
	makeUser(t, email, role)

	// A negative TTL hands out a token that is already expired but still well
	// inside the refresh window, which is exactly the case refresh has to
	// tolerate. Waiting out a real JWT_TTL would cost a minute per run.
	previousTTL := facades.Config().GetInt("jwt.ttl")
	facades.Config().Add("jwt.ttl", -1)
	expired := login(t, email)
	facades.Config().Add("jwt.ttl", previousTTL)

	// The expired token is refused everywhere else.
	protected := adminRequest(t, http.MethodGet, "/admin/dashboard", nil, expired)
	require.Equal(t, http.StatusUnauthorized, protected.Code, describe(protected))

	refreshed := adminRequest(t, http.MethodPost, "/admin/auth/refresh", nil, expired)
	require.Equal(t, http.StatusOK, refreshed.Code, describe(refreshed))

	fresh, ok := decode(t, refreshed)["token"].(string)
	require.True(t, ok)

	// The new token works on a protected route.
	after := adminRequest(t, http.MethodGet, "/admin/dashboard", nil, "Bearer "+fresh)
	require.Equal(t, http.StatusOK, after.Code, describe(after))

	// Garbage is still a 401.
	for _, token := range []string{"", "Bearer not-a-jwt"} {
		recorder := adminRequest(t, http.MethodPost, "/admin/auth/refresh", nil, token)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, describe(recorder))
	}
}

func TestAdminRefreshRejectsTokenBeyondRefreshTTL(t *testing.T) {
	email := uniqueEmail(t)
	role := makeRole(t, "refresh_ttl_role", []string{"admin.dashboard.index"}, true)
	makeUser(t, email, role)

	// Expired longer ago than the refresh window is wide: goravel's Refresh()
	// compares the claims' expiry plus jwt.refresh_ttl against now, so this
	// token must not be refreshable any more.
	refreshTTL := facades.Config().GetInt("jwt.refresh_ttl")
	previousTTL := facades.Config().GetInt("jwt.ttl")
	facades.Config().Add("jwt.ttl", -(refreshTTL + 60))
	stale := login(t, email)
	facades.Config().Add("jwt.ttl", previousTTL)

	recorder := adminRequest(t, http.MethodPost, "/admin/auth/refresh", nil, stale)
	require.Equal(t, http.StatusUnauthorized, recorder.Code, describe(recorder))
}

func TestCanUserForbidsRoleWithoutPermission(t *testing.T) {
	email := uniqueEmail(t)
	role := makeRole(t, "limited", []string{"admin.auth.me"}, true)
	makeUser(t, email, role)

	token := login(t, email)

	allowed := adminRequest(t, http.MethodGet, "/admin/auth/me", nil, token)
	require.Equal(t, http.StatusOK, allowed.Code, describe(allowed))

	forbidden := adminRequest(t, http.MethodGet, "/admin/dashboard", nil, token)
	require.Equal(t, http.StatusForbidden, forbidden.Code, describe(forbidden))
}

func TestClientCreateDuplicateThenRecreateAfterSoftDelete(t *testing.T) {
	email := uniqueEmail(t)
	role := makeRole(t, "clients_admin", []string{
		"admin.clients.store",
		"admin.clients.destroy",
	}, true)
	makeUser(t, email, role)

	token := login(t, email)
	username := fmt.Sprintf("dup-client-%d", time.Now().UnixNano())

	created := adminRequest(t, http.MethodPost, "/admin/clients",
		map[string]any{"name": "dup", "username": username}, token)
	require.Equal(t, http.StatusCreated, created.Code, describe(created))

	firstID := clientIDOf(t, created)
	t.Cleanup(func() { forceDeleteClientByID(firstID) })

	duplicate := adminRequest(t, http.MethodPost, "/admin/clients",
		map[string]any{"name": "dup", "username": username}, token)
	require.Equal(t, http.StatusUnprocessableEntity, duplicate.Code, describe(duplicate))

	deleted := adminRequest(t, http.MethodDelete, fmt.Sprintf("/admin/clients/%d", firstID), nil, token)
	require.Equal(t, http.StatusNoContent, deleted.Code, describe(deleted))

	// The partial unique index only covers the live rows, so the name is free
	// again the moment the old client is in the trash.
	recreated := adminRequest(t, http.MethodPost, "/admin/clients",
		map[string]any{"name": "dup", "username": username}, token)
	require.Equal(t, http.StatusCreated, recreated.Code, describe(recreated))

	secondID := clientIDOf(t, recreated)
	t.Cleanup(func() { forceDeleteClientByID(secondID) })
	require.NotEqual(t, firstID, secondID)
}

func TestBlockedClientLosesAccessImmediately(t *testing.T) {
	client := makeClient(t, fmt.Sprintf("blocked-%d", time.Now().UnixNano()), nil)

	first := jsonRequest(t, client, http.MethodGet, "/api/me")
	require.Equal(t, http.StatusOK, first.Code, describe(first))

	// The middleware caches the client row and the password digest, so the
	// status change only takes effect if the service drops both.
	status := "blocked"
	require.NoError(t, services.NewClientService().Update(client, services.ClientInput{Status: &status}, nil))

	after := jsonRequest(t, client, http.MethodGet, "/api/me")
	require.Equal(t, http.StatusForbidden, after.Code, describe(after))
}

func TestOversizedUploadIsRejectedBeforeItIsParsed(t *testing.T) {
	client := makeClient(t, fmt.Sprintf("toobig-%d", time.Now().UnixNano()), nil)

	// Shrink the hard cap instead of sending a gigabyte: the raw wrapper
	// recomputes it from the configuration on every request.
	previous := facades.Config().GetInt("storage.max_file_size")
	facades.Config().Add("storage.max_file_size", 1024)
	t.Cleanup(func() { facades.Config().Add("storage.max_file_size", previous) })

	before := tempUploadCount(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files[]", "big.bin")
	require.NoError(t, err)
	_, err = part.Write(randomBytes(3 << 20))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/api/files", bytes.NewReader(body.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	for key, value := range clientHeaders(client) {
		request.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	facades.Route().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code, describe(recorder))
	require.Equal(t, before, tempUploadCount(t),
		"the body must be refused before the multipart parser spills it to disk")

	// Nothing was stored either.
	count, err := facades.Orm().Query().Model(&models.File{}).Where("client_id", client.ID).Count()
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

// clientIDOf reads the id out of a client creation response.
func clientIDOf(t *testing.T, recorder *httptest.ResponseRecorder) uint {
	t.Helper()

	body := decode(t, recorder)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, describe(recorder))

	id, ok := data["id"].(float64)
	require.True(t, ok, describe(recorder))

	return uint(id)
}

// forceDeleteClientByID purges a client the API created, trashed rows included.
func forceDeleteClientByID(id uint) {
	var client models.Client
	if err := facades.Orm().Query().Model(&models.Client{}).WithTrashed().
		Where("id", id).First(&client); err != nil || client.ID == 0 {
		return
	}

	removeClient(&client)
}

func TestPublicStaticFilesCannotExecute(t *testing.T) {
	client := makeClient(t, fmt.Sprintf("static-%d", time.Now().UnixNano()), nil)

	recorder := uploadRequest(t, client, nil, []uploadPart{
		{Field: "files[]", Name: "page.html", Content: []byte("<html><script>alert(1)</script></html>")},
		{Field: "files[]", Name: "note.txt", Content: []byte("hello")},
	})
	require.Equal(t, http.StatusCreated, recorder.Code, describe(recorder))

	var files []models.File
	require.NoError(t, facades.Orm().Query().Model(&models.File{}).
		Where("client_id", client.ID).OrderBy("id").Get(&files))
	require.Len(t, files, 2)

	for _, file := range files {
		served := httptest.NewRecorder()
		facades.Route().ServeHTTP(served, httptest.NewRequest(http.MethodGet, "/storage/"+file.Path, nil))

		require.Equal(t, http.StatusOK, served.Code, describe(served))
		require.Equal(t, "nosniff", served.Header().Get("X-Content-Type-Options"))

		if file.Extension == "html" {
			require.Contains(t, served.Header().Get("Content-Disposition"), "attachment")
			require.Equal(t, "sandbox", served.Header().Get("Content-Security-Policy"))
		} else {
			require.Equal(t, "inline", served.Header().Get("Content-Disposition"))
			require.Empty(t, served.Header().Get("Content-Security-Policy"))
		}
	}
}
