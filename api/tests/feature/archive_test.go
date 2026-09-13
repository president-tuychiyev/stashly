package feature

import (
	"archive/zip"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/jobs"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
	"github.com/president-tuychiyev/stashly/api/tests"
)

type ArchiveTestSuite struct {
	suite.Suite
	tests.TestCase
}

func TestArchiveTestSuite(t *testing.T) {
	suite.Run(t, new(ArchiveTestSuite))
}

func (s *ArchiveTestSuite) TestZipJobProducesValidZip() {
	client := makeClient(s.T(), "test-archive-zip", nil)

	first := uploadRequest(s.T(), client, map[string]string{"folder": "docs"}, []uploadPart{
		{Field: "file", Name: "report.txt", Content: []byte("report body")},
	})
	s.Require().Equal(http.StatusCreated, first.Code, describe(first))

	second := uploadRequest(s.T(), client, map[string]string{"folder": "avatars", "visibility": "private"}, []uploadPart{
		{Field: "file", Name: "face.png", Content: onePixelPNG},
	})
	s.Require().Equal(http.StatusCreated, second.Code, describe(second))

	archive, err := services.NewArchiveService().Create(
		client.ID, []string{"docs", "avatars"}, nil, models.ActorTypeClient, client.ID,
	)
	s.Require().NoError(err)

	// Run the job in line instead of waiting for a queue worker.
	s.Require().NoError((&jobs.ZipFolders{}).Handle(archive.ID))

	var finished models.Archive
	s.Require().NoError(facades.Orm().Query().Where("id", archive.ID).First(&finished))
	s.Equal(models.ArchiveStatusDone, finished.Status)
	s.Equal(100, finished.Progress)
	s.Equal(2, finished.FilesCount)
	s.Require().NotNil(finished.Size)
	s.Positive(*finished.Size)
	s.Require().NotNil(finished.ExpiresAt)
	s.Require().NotNil(finished.FinishedAt)

	reader, err := zip.OpenReader(zipPath(s.T(), &finished))
	s.Require().NoError(err)
	defer reader.Close()

	names := make([]string, 0, len(reader.File))
	for _, entry := range reader.File {
		names = append(names, entry.Name)
	}
	s.ElementsMatch([]string{"docs/report.txt", "avatars/face.png"}, names)

	// The zip must carry the real bytes, both public and private ones.
	for _, entry := range reader.File {
		handle, openErr := entry.Open()
		s.Require().NoError(openErr)
		buffer := make([]byte, entry.UncompressedSize64)
		read, _ := handle.Read(buffer)
		handle.Close()
		s.Positive(read)
	}
}

func (s *ArchiveTestSuite) TestZipJobMarksFailureForMissingArchive() {
	err := (&jobs.ZipFolders{}).Handle(uint(0))
	s.Error(err)
}

func (s *ArchiveTestSuite) TestArchiveDownloadIsNotAvailableBeforeItIsDone() {
	client := makeClient(s.T(), "test-archive-pending", nil)

	archive := &models.Archive{
		ClientID:      client.ID,
		CreatedByType: models.ActorTypeClient,
		CreatedByID:   client.ID,
		Folders:       []string{"*"},
		Status:        models.ArchiveStatusPending,
	}
	s.Require().NoError(facades.Orm().Query().Create(archive))

	recorder := jsonRequest(s.T(), client, http.MethodGet, "/api/archives/"+itoa(archive.ID)+"/download")
	s.Equal(http.StatusNotFound, recorder.Code, describe(recorder))
}
