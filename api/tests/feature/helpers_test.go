package feature

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// testClientPassword is the plain password every test client is created with.
const testClientPassword = "test-password-123"

// makeClient creates a throw away client and returns it. The client and all of
// its files are removed when the test ends.
func makeClient(t *testing.T, username string, apply func(client *models.Client)) *models.Client {
	t.Helper()

	hashed, err := facades.Hash().Make(testClientPassword)
	require.NoError(t, err)

	name := username
	client := &models.Client{
		Name:         &name,
		Username:     &username,
		Password:     hashed,
		Status:       "active",
		AllowedMimes: []string{},
	}
	if apply != nil {
		apply(client)
	}

	require.NoError(t, facades.Orm().Query().Create(client))

	t.Cleanup(func() {
		removeClient(client)
	})

	return client
}

// removeClient deletes a client together with everything it owns.
func removeClient(client *models.Client) {
	storage := services.NewStorageService()

	var files []models.File
	if err := facades.Orm().Query().Model(&models.File{}).Where("client_id", client.ID).Get(&files); err == nil {
		for index := range files {
			_ = storage.Delete(&files[index])
		}
	}

	var archives []models.Archive
	if err := facades.Orm().Query().Model(&models.Archive{}).Where("client_id", client.ID).Get(&archives); err == nil {
		service := services.NewArchiveService()
		for index := range archives {
			_ = service.Delete(&archives[index])
		}
	}

	_, _ = facades.Orm().Query().Model(&models.Device{}).Where("client_id", client.ID).Delete()
	_, _ = facades.Orm().Query().ForceDelete(client)
	_ = facades.Cache().Forget(services.ClientCacheKey(*client.Username))
	_ = facades.Cache().Forget(services.ClientAuthCacheKey(*client.Username))
}

// clientHeaders builds the header set the api_check middleware expects.
func clientHeaders(client *models.Client) map[string]string {
	return map[string]string{
		"X-Username": *client.Username,
		"X-Password": testClientPassword,
		"X-Language": "en",
		"X-Device":   "test-device-" + *client.Username,
		"X-Platform": "web",
	}
}

// uploadPart describes one file of a multipart upload.
type uploadPart struct {
	Field   string
	Name    string
	Content []byte
}

// uploadRequest performs a multipart POST against the running route stack.
func uploadRequest(t *testing.T, client *models.Client, fields map[string]string, parts []uploadPart) *httptest.ResponseRecorder {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	for _, part := range parts {
		file, err := writer.CreateFormFile(part.Field, part.Name)
		require.NoError(t, err)
		_, err = file.Write(part.Content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/api/files", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	for key, value := range clientHeaders(client) {
		request.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	facades.Route().ServeHTTP(recorder, request)

	return recorder
}

// jsonRequest performs a GET or DELETE against the running route stack.
func jsonRequest(t *testing.T, client *models.Client, method, target string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, target, nil)
	for key, value := range clientHeaders(client) {
		request.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	facades.Route().ServeHTTP(recorder, request)

	return recorder
}

// storedPath returns the absolute location of a stored file.
func storedPath(t *testing.T, file *models.File) string {
	t.Helper()

	target, err := services.NewStorageService().AbsolutePath(file)
	require.NoError(t, err)

	return target
}

// fileExists reports whether a path is a readable file.
func fileExists(path string) bool {
	info, err := os.Stat(path)

	return err == nil && !info.IsDir()
}

// randomBytes builds deterministic filler content of the requested size.
func randomBytes(size int) []byte {
	out := make([]byte, size)
	for index := range out {
		out[index] = byte(index % 251)
	}

	return out
}

// onePixelPNG is the smallest valid PNG, used to test MIME detection.
var onePixelPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41, 0x54, 0x78, 0xDA, 0x63, 0xFC, 0xCF, 0xC0, 0x50,
	0x0F, 0x00, 0x04, 0x85, 0x01, 0x80, 0x84, 0xA9, 0x8C, 0x21, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

// describe renders a recorder for assertion messages.
func describe(recorder *httptest.ResponseRecorder) string {
	return fmt.Sprintf("status %d body %s", recorder.Code, recorder.Body.String())
}

// zipPath is the on-disk location of a finished archive.
func zipPath(t *testing.T, archive *models.Archive) string {
	t.Helper()

	require.NotNil(t, archive.Path)
	target, err := services.NewStorageService().ResolvePath("archives", filepath.Base(*archive.Path))
	require.NoError(t, err)

	return target
}

// tempUploadCount counts the copies goravel leaves in the system temp dir.
func tempUploadCount(t *testing.T) int {
	t.Helper()

	entries, err := os.ReadDir(os.TempDir())
	require.NoError(t, err)

	prefix := facades.Config().GetString("app.name") + "_"
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			count++
		}
	}

	return count
}
