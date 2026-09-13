package feature

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
	"github.com/president-tuychiyev/stashly/api/tests"
)

type ScopingTestSuite struct {
	suite.Suite
	tests.TestCase
}

func TestScopingTestSuite(t *testing.T) {
	suite.Run(t, new(ScopingTestSuite))
}

// itoa renders an unsigned id for use in a URL.
func itoa(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}

func (s *ScopingTestSuite) TestClientsCannotReachEachOthersFiles() {
	owner := makeClient(s.T(), "test-scope-owner", nil)
	stranger := makeClient(s.T(), "test-scope-stranger", nil)

	upload := uploadRequest(s.T(), owner, map[string]string{"folder": "docs"}, []uploadPart{
		{Field: "file", Name: "secret.txt", Content: []byte("owner only")},
	})
	s.Require().Equal(http.StatusCreated, upload.Code, describe(upload))

	var payload struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	s.Require().NoError(json.Unmarshal(upload.Body.Bytes(), &payload))
	s.Require().Len(payload.Data, 1)
	fileID := itoa(payload.Data[0].ID)

	// The owner can read its own file.
	own := jsonRequest(s.T(), owner, http.MethodGet, "/api/files/"+fileID)
	s.Equal(http.StatusOK, own.Code, describe(own))

	// Another client gets 404, never 403, so ids stay unguessable.
	for _, target := range []string{"/api/files/" + fileID, "/api/files/" + fileID + "/download"} {
		recorder := jsonRequest(s.T(), stranger, http.MethodGet, target)
		s.Equal(http.StatusNotFound, recorder.Code, "%s: %s", target, describe(recorder))
	}

	deletion := jsonRequest(s.T(), stranger, http.MethodDelete, "/api/files/"+fileID)
	s.Equal(http.StatusNotFound, deletion.Code, describe(deletion))

	// The listing of a stranger stays empty.
	listing := jsonRequest(s.T(), stranger, http.MethodGet, "/api/files")
	s.Require().Equal(http.StatusOK, listing.Code, describe(listing))

	var list struct {
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	s.Require().NoError(json.Unmarshal(listing.Body.Bytes(), &list))
	s.Zero(list.Meta.Total)

	// And the file is still there for its owner.
	after := jsonRequest(s.T(), owner, http.MethodGet, "/api/files/"+fileID)
	s.Equal(http.StatusOK, after.Code, describe(after))
}

func (s *ScopingTestSuite) TestFolderListingIsScopedToTheClient() {
	owner := makeClient(s.T(), "test-scope-folders-a", nil)
	other := makeClient(s.T(), "test-scope-folders-b", nil)

	upload := uploadRequest(s.T(), owner, map[string]string{"folder": "reports"}, []uploadPart{
		{Field: "file", Name: "a.txt", Content: []byte("a")},
	})
	s.Require().Equal(http.StatusCreated, upload.Code, describe(upload))

	mine := jsonRequest(s.T(), owner, http.MethodGet, "/api/folders")
	s.Require().Equal(http.StatusOK, mine.Code, describe(mine))
	s.Contains(mine.Body.String(), "reports")

	theirs := jsonRequest(s.T(), other, http.MethodGet, "/api/folders")
	s.Require().Equal(http.StatusOK, theirs.Code, describe(theirs))
	s.NotContains(theirs.Body.String(), "reports")
}

// makeOwnedClient creates a client that belongs to one admin user.
func (s *ScopingTestSuite) makeOwnedClient(owner *models.User, username string) *models.Client {
	return makeClient(s.T(), username, func(client *models.Client) {
		client.OwnerID = &owner.ID
	})
}

func (s *ScopingTestSuite) TestAdminsCannotReachEachOthersClients() {
	mine := makeAdmin(s.T(), uniqueEmail(s.T()))
	theirs := makeAdmin(s.T(), uniqueEmail(s.T()))

	suffix := time.Now().UnixNano()
	own := s.makeOwnedClient(mine, fmt.Sprintf("own-%d", suffix))
	foreign := s.makeOwnedClient(theirs, fmt.Sprintf("foreign-%d", suffix))

	token := login(s.T(), mine.Email)

	// The client of another admin is missing, not forbidden.
	visible := adminRequest(s.T(), http.MethodGet, "/admin/clients/"+itoa(own.ID), nil, token)
	s.Require().Equal(http.StatusOK, visible.Code, describe(visible))

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		recorder := adminRequest(s.T(), method, "/admin/clients/"+itoa(foreign.ID), nil, token)
		s.Equal(http.StatusNotFound, recorder.Code, "%s: %s", method, describe(recorder))
	}

	reset := adminRequest(s.T(), http.MethodPost,
		"/admin/clients/"+itoa(foreign.ID)+"/reset-password", nil, token)
	s.Equal(http.StatusNotFound, reset.Code, describe(reset))

	devices := adminRequest(s.T(), http.MethodGet, "/admin/clients/"+itoa(foreign.ID)+"/devices", nil, token)
	s.Equal(http.StatusNotFound, devices.Code, describe(devices))

	// And the listing only carries what it owns.
	listing := adminRequest(s.T(), http.MethodGet, "/admin/clients?per_page=100", nil, token)
	s.Require().Equal(http.StatusOK, listing.Code, describe(listing))
	s.Contains(listing.Body.String(), *own.Username)
	s.NotContains(listing.Body.String(), *foreign.Username)
}

func (s *ScopingTestSuite) TestAdminsCannotReachEachOthersFilesAndArchives() {
	mine := makeAdmin(s.T(), uniqueEmail(s.T()))
	theirs := makeAdmin(s.T(), uniqueEmail(s.T()))

	suffix := time.Now().UnixNano()
	own := s.makeOwnedClient(mine, fmt.Sprintf("files-own-%d", suffix))
	foreign := s.makeOwnedClient(theirs, fmt.Sprintf("files-foreign-%d", suffix))

	upload := uploadRequest(s.T(), foreign, nil, []uploadPart{
		{Field: "file", Name: "theirs.txt", Content: []byte("not yours")},
	})
	s.Require().Equal(http.StatusCreated, upload.Code, describe(upload))

	var stored models.File
	s.Require().NoError(facades.Orm().Query().Model(&models.File{}).
		Where("client_id", foreign.ID).First(&stored))

	archive, err := services.NewArchiveService().
		Create(foreign.ID, []string{"*"}, nil, models.ActorTypeUser, theirs.ID)
	s.Require().NoError(err)

	token := login(s.T(), mine.Email)

	for _, target := range []string{
		"/admin/files/" + itoa(stored.ID),
		"/admin/files/" + itoa(stored.ID) + "/download",
		"/admin/archives/" + itoa(archive.ID),
		"/admin/archives/" + itoa(archive.ID) + "/download",
		"/admin/folders?client_id=" + itoa(foreign.ID),
	} {
		recorder := adminRequest(s.T(), http.MethodGet, target, nil, token)
		s.Equal(http.StatusNotFound, recorder.Code, "%s: %s", target, describe(recorder))
	}

	deletion := adminRequest(s.T(), http.MethodDelete, "/admin/files/"+itoa(stored.ID), nil, token)
	s.Equal(http.StatusNotFound, deletion.Code, describe(deletion))

	// A bulk delete silently skips what the caller does not own.
	bulk := adminRequest(s.T(), http.MethodPost, "/admin/files/bulk-delete",
		map[string]any{"ids": []uint{stored.ID}}, token)
	s.Require().Equal(http.StatusOK, bulk.Code, describe(bulk))
	s.Equal(float64(0), decode(s.T(), bulk)["deleted"])

	// Uploading into somebody else's client is refused too.
	uploaded := adminRequest(s.T(), http.MethodPost, "/admin/archives",
		map[string]any{"client_id": foreign.ID, "folders": []string{"*"}}, token)
	s.Equal(http.StatusUnprocessableEntity, uploaded.Code, describe(uploaded))

	// The listings stay empty for the other admin.
	for _, target := range []string{"/admin/files?per_page=100", "/admin/archives?per_page=100"} {
		listing := adminRequest(s.T(), http.MethodGet, target, nil, token)
		s.Require().Equal(http.StatusOK, listing.Code, describe(listing))
		s.NotContains(listing.Body.String(), stored.Name)
	}

	_ = own
}

func (s *ScopingTestSuite) TestDashboardCountsOnlyOwnedClients() {
	mine := makeAdmin(s.T(), uniqueEmail(s.T()))
	theirs := makeAdmin(s.T(), uniqueEmail(s.T()))

	suffix := time.Now().UnixNano()
	own := s.makeOwnedClient(mine, fmt.Sprintf("dash-own-%d", suffix))
	foreign := s.makeOwnedClient(theirs, fmt.Sprintf("dash-foreign-%d", suffix))

	for _, client := range []*models.Client{own, foreign} {
		upload := uploadRequest(s.T(), client, nil, []uploadPart{
			{Field: "file", Name: "one.txt", Content: []byte("hello")},
		})
		s.Require().Equal(http.StatusCreated, upload.Code, describe(upload))
	}

	token := login(s.T(), mine.Email)

	recorder := adminRequest(s.T(), http.MethodGet, "/admin/dashboard", nil, token)
	s.Require().Equal(http.StatusOK, recorder.Code, describe(recorder))

	data, ok := decode(s.T(), recorder)["data"].(map[string]any)
	s.Require().True(ok, describe(recorder))
	s.Equal(float64(1), data["clients_count"])
	s.Equal(float64(1), data["files_count"])
	s.NotContains(recorder.Body.String(), *foreign.Username)
}

func (s *ScopingTestSuite) TestStorageSyncIsSuperAdminOnly() {
	admin := makeAdmin(s.T(), uniqueEmail(s.T()))
	super := makeSuperAdmin(s.T(), uniqueEmail(s.T()))

	refused := adminRequest(s.T(), http.MethodPost, "/admin/storage/sync", nil, login(s.T(), admin.Email))
	s.Equal(http.StatusForbidden, refused.Code, describe(refused))

	allowed := adminRequest(s.T(), http.MethodPost, "/admin/storage/sync", nil, login(s.T(), super.Email))
	s.Equal(http.StatusOK, allowed.Code, describe(allowed))
}

func (s *ScopingTestSuite) TestSuperAdminSeesTheOwnerOfAClient() {
	super := makeSuperAdmin(s.T(), uniqueEmail(s.T()))
	owner := makeAdmin(s.T(), uniqueEmail(s.T()))
	token := login(s.T(), super.Email)

	username := fmt.Sprintf("owned-by-%d", time.Now().UnixNano())
	created := adminRequest(s.T(), http.MethodPost, "/admin/clients",
		map[string]any{"name": "owned", "username": username, "owner_id": owner.ID}, token)
	s.Require().Equal(http.StatusCreated, created.Code, describe(created))

	id := clientIDOf(s.T(), created)
	s.T().Cleanup(func() { forceDeleteClientByID(id) })

	shown := adminRequest(s.T(), http.MethodGet, "/admin/clients/"+itoa(id), nil, token)
	s.Require().Equal(http.StatusOK, shown.Code, describe(shown))

	data, ok := decode(s.T(), shown)["data"].(map[string]any)
	s.Require().True(ok, describe(shown))
	s.Equal(float64(owner.ID), data["owner_id"])

	rendered, ok := data["owner"].(map[string]any)
	s.Require().True(ok, describe(shown))
	s.Equal(owner.Email, rendered["email"])
}

func (s *ScopingTestSuite) TestAdminOwnsWhatItCreates() {
	admin := makeAdmin(s.T(), uniqueEmail(s.T()))
	other := makeAdmin(s.T(), uniqueEmail(s.T()))
	token := login(s.T(), admin.Email)

	username := fmt.Sprintf("self-owned-%d", time.Now().UnixNano())
	created := adminRequest(s.T(), http.MethodPost, "/admin/clients",
		map[string]any{"name": "mine", "username": username, "owner_id": other.ID}, token)
	s.Require().Equal(http.StatusCreated, created.Code, describe(created))

	id := clientIDOf(s.T(), created)
	s.T().Cleanup(func() { forceDeleteClientByID(id) })

	// The owner_id in the payload is ignored for an admin.
	var stored models.Client
	s.Require().NoError(facades.Orm().Query().Where("id", id).First(&stored))
	s.Require().NotNil(stored.OwnerID)
	s.Equal(admin.ID, *stored.OwnerID)
}
