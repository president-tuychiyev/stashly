package feature

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/tests"
)

type UploadTestSuite struct {
	suite.Suite
	tests.TestCase
}

func TestUploadTestSuite(t *testing.T) {
	suite.Run(t, new(UploadTestSuite))
}

// decode reads the "data" array of an upload response.
func (s *UploadTestSuite) decode(body []byte) []map[string]any {
	var payload struct {
		Data []map[string]any `json:"data"`
	}
	s.Require().NoError(json.Unmarshal(body, &payload))

	return payload.Data
}

func (s *UploadTestSuite) TestSingleUpload() {
	client := makeClient(s.T(), "test-upload-single", nil)
	content := []byte("a short text report\n")

	recorder := uploadRequest(s.T(), client, map[string]string{"folder": "docs/2026"}, []uploadPart{
		{Field: "file", Name: "report.txt", Content: content},
	})

	s.Require().Equal(http.StatusCreated, recorder.Code, describe(recorder))

	data := s.decode(recorder.Body.Bytes())
	s.Require().Len(data, 1)
	s.Equal("report.txt", data[0]["original_name"])
	s.Equal("docs/2026", data[0]["folder"])
	s.Equal("public", data[0]["visibility"])
	s.NotNil(data[0]["url"])

	digest := sha256.Sum256(content)
	s.Equal(hex.EncodeToString(digest[:]), data[0]["sha256"])

	var stored models.File
	s.Require().NoError(facadesOrmFirst(&stored, client.ID))
	s.True(fileExists(storedPath(s.T(), &stored)), "the file should be on disk")
}

func (s *UploadTestSuite) TestMultipleUpload() {
	client := makeClient(s.T(), "test-upload-multiple", nil)

	recorder := uploadRequest(s.T(), client, map[string]string{"folder": "avatars", "visibility": "private"}, []uploadPart{
		{Field: "files", Name: "one.png", Content: onePixelPNG},
		{Field: "files", Name: "two.bin", Content: randomBytes(1024)},
	})

	s.Require().Equal(http.StatusCreated, recorder.Code, describe(recorder))

	data := s.decode(recorder.Body.Bytes())
	s.Require().Len(data, 2)
	for _, file := range data {
		s.Equal("private", file["visibility"])
		s.Nil(file["url"], "private files must not expose a public url")
	}
	s.Equal("image/png", data[0]["mime"])
}

func (s *UploadTestSuite) TestRejectsInvalidFolder() {
	client := makeClient(s.T(), "test-upload-folder", nil)

	for _, folder := range []string{"../etc", "/leading", "UPPER", "a/b/c/d"} {
		recorder := uploadRequest(s.T(), client, map[string]string{"folder": folder}, []uploadPart{
			{Field: "file", Name: "a.txt", Content: []byte("x")},
		})
		s.Equal(http.StatusUnprocessableEntity, recorder.Code, "folder %q should be rejected: %s", folder, describe(recorder))
	}
}

func (s *UploadTestSuite) TestRejectsDisallowedMime() {
	client := makeClient(s.T(), "test-upload-mime", func(client *models.Client) {
		client.AllowedMimes = []string{"image/*"}
	})

	recorder := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "file", Name: "a.txt", Content: []byte("plain text, not an image")},
	})
	s.Equal(http.StatusUnsupportedMediaType, recorder.Code, describe(recorder))

	allowed := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "file", Name: "a.png", Content: onePixelPNG},
	})
	s.Equal(http.StatusCreated, allowed.Code, describe(allowed))
}

func (s *UploadTestSuite) TestRejectsFileTooLarge() {
	limit := int64(512)
	client := makeClient(s.T(), "test-upload-large", func(client *models.Client) {
		client.MaxFileSize = &limit
	})

	recorder := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "file", Name: "big.bin", Content: randomBytes(2048)},
	})

	s.Equal(http.StatusRequestEntityTooLarge, recorder.Code, describe(recorder))
	s.Contains(recorder.Body.String(), "file too large")
}

func (s *UploadTestSuite) TestRejectsQuotaExceeded() {
	quota := int64(1024)
	client := makeClient(s.T(), "test-upload-quota", func(client *models.Client) {
		client.QuotaBytes = &quota
	})

	first := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "file", Name: "a.bin", Content: randomBytes(800)},
	})
	s.Require().Equal(http.StatusCreated, first.Code, describe(first))

	second := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "file", Name: "b.bin", Content: randomBytes(800)},
	})
	s.Equal(http.StatusRequestEntityTooLarge, second.Code, describe(second))
	s.Contains(second.Body.String(), "quota exceeded")
}

func (s *UploadTestSuite) TestMultipleUploadIsAllOrNothing() {
	client := makeClient(s.T(), "test-upload-atomic", func(client *models.Client) {
		client.AllowedMimes = []string{"image/*"}
	})

	recorder := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "files", Name: "ok.png", Content: onePixelPNG},
		{Field: "files", Name: "bad.txt", Content: []byte("not an image at all")},
	})

	s.Equal(http.StatusUnsupportedMediaType, recorder.Code, describe(recorder))
	s.Contains(recorder.Body.String(), "files.1")

	count, err := facadesOrmCount(client.ID)
	s.Require().NoError(err)
	s.Zero(count, "nothing should have been stored")
}

func (s *UploadTestSuite) TestAcceptsContractFilesArrayField() {
	client := makeClient(s.T(), "test-upload-bracket", nil)

	recorder := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "files[]", Name: "one.png", Content: onePixelPNG},
		{Field: "files[]", Name: "two.png", Content: onePixelPNG},
	})

	s.Require().Equal(http.StatusCreated, recorder.Code, describe(recorder))
	s.Require().Len(s.decode(recorder.Body.Bytes()), 2)
}

func (s *UploadTestSuite) TestCollectsAnErrorForEveryOffendingFile() {
	client := makeClient(s.T(), "test-upload-collect", func(client *models.Client) {
		client.AllowedMimes = []string{"image/*"}
	})

	recorder := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "files[]", Name: "ok.png", Content: onePixelPNG},
		{Field: "files[]", Name: "bad.txt", Content: []byte("not an image at all")},
		{Field: "files[]", Name: "worse.txt", Content: []byte("also not an image")},
	})

	s.Require().Equal(http.StatusUnsupportedMediaType, recorder.Code, describe(recorder))

	var payload struct {
		Errors map[string][]string `json:"errors"`
	}
	s.Require().NoError(json.Unmarshal(recorder.Body.Bytes(), &payload))

	s.Equal([]string{"mime not allowed"}, payload.Errors["files.1"])
	s.Equal([]string{"mime not allowed"}, payload.Errors["files.2"], "every offending file must be reported")
	s.NotContains(payload.Errors, "files.0")

	count, err := facadesOrmCount(client.ID)
	s.Require().NoError(err)
	s.Zero(count, "nothing should have been stored")
}

func (s *UploadTestSuite) TestAllowListIgnoresAFakeExtension() {
	client := makeClient(s.T(), "test-upload-fake-ext", func(client *models.Client) {
		client.AllowedMimes = []string{"image/*"}
	})

	// Plain text named .png: the extension says image/png, the content does
	// not, and only the content may decide.
	recorder := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "file", Name: "totally-an-image.png", Content: []byte("#!/bin/sh\nrm -rf /\n")},
	})

	s.Equal(http.StatusUnsupportedMediaType, recorder.Code, describe(recorder))
	s.Contains(recorder.Body.String(), "mime not allowed")
}

func (s *UploadTestSuite) TestExtensionOnlyLabelsTheStoredMimeWithoutAnAllowList() {
	client := makeClient(s.T(), "test-upload-label", nil)

	recorder := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "file", Name: "sheet.csv", Content: randomBytes(64)},
	})

	s.Require().Equal(http.StatusCreated, recorder.Code, describe(recorder))

	data := s.decode(recorder.Body.Bytes())
	s.Require().Len(data, 1)
	s.Contains(data[0]["mime"], "text/csv", "the extension may still label an undetectable file")
}

func (s *UploadTestSuite) TestTooManyFilesReportsTheConfiguredLimit() {
	client := makeClient(s.T(), "test-upload-too-many", nil)

	parts := make([]uploadPart, 0, 21)
	for index := 0; index < 21; index++ {
		parts = append(parts, uploadPart{Field: "files[]", Name: "a.png", Content: onePixelPNG})
	}

	recorder := uploadRequest(s.T(), client, nil, parts)

	s.Equal(http.StatusUnprocessableEntity, recorder.Code, describe(recorder))
	s.Contains(recorder.Body.String(), "at most 20 files per request")
}

func (s *UploadTestSuite) TestUploadLeavesNoTemporaryFilesBehind() {
	client := makeClient(s.T(), "test-upload-temp", nil)

	before := tempUploadCount(s.T())

	recorder := uploadRequest(s.T(), client, nil, []uploadPart{
		{Field: "files[]", Name: "one.png", Content: onePixelPNG},
		{Field: "files[]", Name: "two.png", Content: onePixelPNG},
	})
	s.Require().Equal(http.StatusCreated, recorder.Code, describe(recorder))

	s.Equal(before, tempUploadCount(s.T()), "the goravel temp copies must be removed")

	// A rejected upload must clean up too.
	rejected := uploadRequest(s.T(), client, map[string]string{"folder": "../escape"}, []uploadPart{
		{Field: "files[]", Name: "one.png", Content: onePixelPNG},
	})
	s.Require().Equal(http.StatusUnprocessableEntity, rejected.Code, describe(rejected))

	s.Equal(before, tempUploadCount(s.T()), "a rejected upload must not leak temp copies either")
}
