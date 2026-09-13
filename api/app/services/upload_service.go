package services

import (
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"strconv"
	"strings"

	"github.com/goravel/framework/contracts/database/orm"
	contractsfilesystem "github.com/goravel/framework/contracts/filesystem"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/filesystem"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

// multipartBuffer is how much of a multipart body the parser may keep in
// memory before spilling to a temp file. It matches the gin body_limit.
const multipartBuffer = 32 << 20

// errQuotaExceeded aborts the storing transaction when the locked client row
// turns out to be over quota.
var errQuotaExceeded = errors.New("quota exceeded")

// UploadError carries the HTTP status and payload of a rejected upload.
type UploadError struct {
	Status  int
	Message string
	Fields  map[string][]string
}

type UploadService struct {
	storage *StorageService
	audit   *AuditService
}

func NewUploadService() *UploadService {
	return &UploadService{storage: NewStorageService(), audit: NewAuditService()}
}

// Handle validates and stores the files of a multipart request. Every file is
// checked before anything is written, so a single rejection leaves the disk
// untouched, and every rejection is reported for every offending file at once.
func (r *UploadService) Handle(ctx http.Context, client *models.Client, actor Actor) ([]models.File, *UploadError) {
	folder, err := r.storage.NormalizeFolder(ctx.Request().Input("folder"))
	if err != nil {
		return nil, &UploadError{
			Status: http.StatusUnprocessableEntity,
			Fields: map[string][]string{"folder": {"invalid folder name"}},
		}
	}

	visibility := strings.ToLower(strings.TrimSpace(ctx.Request().Input("visibility", models.VisibilityPublic)))
	if visibility == "" {
		visibility = models.VisibilityPublic
	}
	if visibility != models.VisibilityPublic && visibility != models.VisibilityPrivate {
		return nil, &UploadError{
			Status: http.StatusUnprocessableEntity,
			Fields: map[string][]string{"visibility": {"visibility must be public or private"}},
		}
	}

	headers, base, uploadErr := r.headers(ctx)
	if uploadErr != nil {
		return nil, uploadErr
	}

	maxFileSize := r.storage.MaxFileSize(client)

	// First pass: the size announced in the multipart header. This runs before
	// anything is copied anywhere, so an oversized part never reaches a temp
	// file at all.
	collector := newFieldErrors(base, len(headers))
	for index, header := range headers {
		if header.Size > maxFileSize {
			collector.add(index, http.StatusRequestEntityTooLarge, "file too large")
		}
	}
	if uploadErr := collector.result(); uploadErr != nil {
		return nil, uploadErr
	}

	// Second pass: materialise the parts. goravel copies each one into
	// os.TempDir() and never cleans up after itself, so every temp file is
	// removed here whether the request succeeds or fails.
	uploads := make([]contractsfilesystem.File, 0, len(headers))
	defer func() {
		for _, upload := range uploads {
			if path := upload.File(); path != "" {
				_ = os.Remove(path)
			}
		}
	}()

	for _, header := range headers {
		upload, fileErr := filesystem.NewFileFromRequest(header)
		if fileErr != nil {
			return nil, &UploadError{Status: http.StatusInternalServerError, Message: fileErr.Error()}
		}
		uploads = append(uploads, upload)
	}

	// Third pass: inspect the real content of every file and collect all the
	// per file problems before a single byte is written to a disk.
	used, err := r.storage.UsedBytes(client.ID)
	if err != nil {
		return nil, &UploadError{Status: http.StatusInternalServerError, Message: err.Error()}
	}

	candidates := make([]Candidate, 0, len(uploads))
	incoming := int64(0)

	for index, upload := range uploads {
		candidate, inspectErr := r.storage.Inspect(upload, headers[index].Filename)
		if inspectErr != nil {
			return nil, &UploadError{Status: http.StatusInternalServerError, Message: inspectErr.Error()}
		}

		if candidate.Size > maxFileSize {
			collector.add(index, http.StatusRequestEntityTooLarge, "file too large")
		}
		if !r.storage.MimeAllowed(candidate.Mime, client.AllowedMimes) {
			collector.add(index, http.StatusUnsupportedMediaType, "mime not allowed")
		}

		incoming += candidate.Size
		if client.QuotaBytes != nil && used+incoming > *client.QuotaBytes {
			collector.add(index, http.StatusRequestEntityTooLarge, "quota exceeded")
		}

		candidates = append(candidates, candidate)
	}

	if uploadErr := collector.result(); uploadErr != nil {
		return nil, uploadErr
	}

	stored, uploadErr := r.store(client, folder, visibility, base, incoming, candidates)
	if uploadErr != nil {
		return nil, uploadErr
	}

	for index := range stored {
		file := &stored[index]
		id := file.ID
		clientID := file.ClientID
		r.audit.LogFor(actor, "file.upload", "file", &id, &clientID, map[string]any{
			"path":       file.Path,
			"size":       file.Size,
			"mime":       file.Mime,
			"visibility": file.Visibility,
		}, ctx.Request().Ip())
	}

	return stored, nil
}

// store writes the inspected uploads to their final location and then inserts
// their rows.
//
// The copy deliberately happens outside the transaction. The quota check needs
// the client row locked FOR UPDATE, and holding that lock while gigabytes are
// copied to disk would serialise every upload of that client behind the
// slowest one, on a lock that is also taken by the admin endpoints. So the
// files are written first, then the row is locked, the quota is re-checked
// against the sizes that actually landed, the rows are inserted and the lock
// is released again. If anything goes wrong after the copy, every file this
// call wrote is unlinked again, so a rollback still leaves no orphans behind.
func (r *UploadService) store(client *models.Client, folder, visibility, base string, incoming int64, candidates []Candidate) ([]models.File, *UploadError) {
	records := make([]*models.File, 0, len(candidates))
	written := make([]string, 0, len(candidates))

	unlink := func() {
		for _, target := range written {
			_ = os.Remove(target)
		}
	}

	for _, candidate := range candidates {
		// The allow-list is read from the client as it was loaded for this
		// request; the locked row below is only consulted for the quota, which
		// is the one value two concurrent uploads can race on.
		record, target, writeErr := r.storage.Write(
			client.ID, folder, visibility, candidate.StoredMime(client.AllowedMimes), candidate,
		)
		if writeErr != nil {
			unlink()

			return nil, &UploadError{Status: http.StatusInternalServerError, Message: writeErr.Error()}
		}

		records = append(records, record)
		written = append(written, target)
	}

	stored := make([]models.File, 0, len(records))
	offender := len(candidates) - 1

	err := facades.Orm().Transaction(func(tx orm.Query) error {
		stored = stored[:0]

		var locked models.Client
		if err := tx.Where("id", client.ID).LockForUpdate().First(&locked); err != nil {
			return err
		}
		if locked.ID == 0 {
			return errors.New("client not found")
		}

		if locked.QuotaBytes != nil {
			var used *int64
			if err := tx.Model(&models.File{}).Where("client_id", client.ID).Sum("size", &used); err != nil {
				return err
			}
			current := int64(0)
			if used != nil {
				current = *used
			}
			if current+incoming > *locked.QuotaBytes {
				offender = quotaOffender(current, *locked.QuotaBytes, records)

				return errQuotaExceeded
			}
		}

		for _, record := range records {
			if createErr := tx.Create(record); createErr != nil {
				return createErr
			}
			stored = append(stored, *record)
		}

		return nil
	})
	if err != nil {
		unlink()

		if errors.Is(err, errQuotaExceeded) {
			field := fieldName(base, len(candidates), offender)

			return nil, &UploadError{
				Status:  http.StatusRequestEntityTooLarge,
				Message: "quota exceeded",
				Fields:  map[string][]string{field: {"quota exceeded"}},
			}
		}

		return nil, &UploadError{Status: http.StatusInternalServerError, Message: err.Error()}
	}

	return stored, nil
}

// quotaOffender names the file that pushed the client over its quota, so a
// multiple upload blames the part that actually crossed the line instead of
// whichever one happened to be last.
func quotaOffender(used, quota int64, records []*models.File) int {
	running := used
	for index, record := range records {
		running += record.Size
		if running > quota {
			return index
		}
	}

	return len(records) - 1
}

// headers returns the raw multipart headers of the upload, together with the
// base field name used to report per file errors. The contract names the
// multiple upload field "files[]"; "files" and "file" are accepted too.
func (r *UploadService) headers(ctx http.Context) ([]*multipart.FileHeader, string, *UploadError) {
	maxFiles := facades.Config().GetInt("storage.max_files_per_request")
	if maxFiles <= 0 {
		maxFiles = 20
	}

	request := ctx.Request().Origin()
	if request == nil {
		return nil, "", missingFile()
	}
	if request.MultipartForm == nil {
		if err := request.ParseMultipartForm(multipartBuffer); err != nil {
			return nil, "", missingFile()
		}
	}
	if request.MultipartForm == nil {
		return nil, "", missingFile()
	}

	for _, field := range []string{"files[]", "files"} {
		if headers := request.MultipartForm.File[field]; len(headers) > 0 {
			if len(headers) > maxFiles {
				return nil, "", &UploadError{
					Status: http.StatusUnprocessableEntity,
					Fields: map[string][]string{
						"files": {fmt.Sprintf("at most %d files per request", maxFiles)},
					},
				}
			}

			return headers, "files", nil
		}
	}

	if headers := request.MultipartForm.File["file"]; len(headers) > 0 {
		return headers[:1], "file", nil
	}

	return nil, "", missingFile()
}

func missingFile() *UploadError {
	return &UploadError{
		Status: http.StatusUnprocessableEntity,
		Fields: map[string][]string{"file": {"a file is required"}},
	}
}

// fieldErrors accumulates the per file problems of one request so that a
// multiple upload reports every offending file at once instead of stopping at
// the first one.
type fieldErrors struct {
	base   string
	total  int
	status int
	first  string
	fields map[string][]string
}

func newFieldErrors(base string, total int) *fieldErrors {
	return &fieldErrors{base: base, total: total, fields: map[string][]string{}}
}

func (f *fieldErrors) add(index, status int, message string) {
	field := fieldName(f.base, f.total, index)

	for _, existing := range f.fields[field] {
		if existing == message {
			return
		}
	}
	f.fields[field] = append(f.fields[field], message)

	// 413 outranks 415: a file that is both too large and of the wrong type is
	// reported with the status the contract lists first.
	if f.status == 0 || (status == http.StatusRequestEntityTooLarge && f.status != http.StatusRequestEntityTooLarge) {
		f.status = status
		f.first = message
	}
}

func (f *fieldErrors) result() *UploadError {
	if len(f.fields) == 0 {
		return nil
	}

	fields := make(map[string][]string, len(f.fields))
	for key, value := range f.fields {
		fields[key] = value
	}

	return &UploadError{Status: f.status, Message: f.first, Fields: fields}
}

// fieldName names the offending field in a validation error.
func fieldName(base string, total, index int) string {
	if base == "file" && total <= 1 {
		return "file"
	}

	return base + "." + strconv.Itoa(index)
}

// ApplySort turns a "-created_at" style parameter into an order clause. Only
// the listed columns are accepted so the parameter cannot reach the database
// as raw SQL.
func ApplySort(query orm.Query, sort string, allowed []string, fallback string) orm.Query {
	column := strings.TrimSpace(sort)
	descending := strings.HasPrefix(column, "-")
	column = strings.TrimPrefix(column, "-")

	permitted := false
	for _, candidate := range allowed {
		if candidate == column {
			permitted = true
			break
		}
	}

	if !permitted {
		return query.OrderByDesc(fallback)
	}

	if descending {
		return query.OrderByDesc(column)
	}

	return query.OrderBy(column)
}
