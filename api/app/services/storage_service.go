// Package services holds the business logic shared by the HTTP controllers,
// the console commands and the queued jobs.
package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goravel/framework/contracts/database/orm"
	contractsfilesystem "github.com/goravel/framework/contracts/filesystem"
	"github.com/goravel/framework/support/path"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

var (
	// folderPattern accepts up to three lowercase path segments.
	folderPattern = regexp.MustCompile(`^[a-z0-9_-]+(/[a-z0-9_-]+){0,2}$`)

	ErrInvalidFolder = errors.New("invalid folder name")
	ErrOutsideRoot   = errors.New("resolved path escapes the disk root")
)

// Candidate is an upload that has been inspected but not yet written to disk.
type Candidate struct {
	File         contractsfilesystem.File
	OriginalName string
	// Mime is what the content itself says it is. This, and only this, is what
	// the allow-list is checked against.
	Mime string
	// ExtensionMime is what the file name claims. It is never used for the
	// allow-list, only as a nicer label for the stored record when the client
	// has no allow-list at all.
	ExtensionMime string
	Size          int64
}

// StoredMime is the mime recorded on the file row. The extension is only
// trusted to refine an undetectable "application/octet-stream" when the client
// runs without an allow-list; with an allow-list in place the detected type is
// kept verbatim so a renamed file cannot slip past it.
func (c Candidate) StoredMime(allowed []string) string {
	if len(allowed) == 0 && c.Mime == "application/octet-stream" && c.ExtensionMime != "" {
		return c.ExtensionMime
	}

	return c.Mime
}

type StorageService struct{}

func NewStorageService() *StorageService {
	return &StorageService{}
}

// DiskRoot returns the absolute root directory of a disk.
func (r *StorageService) DiskRoot(disk string) string {
	if root := facades.Config().GetString("filesystems.disks." + disk + ".root"); root != "" {
		return root
	}

	return path.Storage("app/" + disk)
}

// ResolvePath joins a relative path onto a disk root and refuses anything that
// would end up outside of that root.
func (r *StorageService) ResolvePath(disk, relative string) (string, error) {
	root := r.DiskRoot(disk)
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	full, err := filepath.Abs(filepath.Join(cleanRoot, filepath.Clean("/"+relative)))
	if err != nil {
		return "", err
	}

	if full != cleanRoot && !strings.HasPrefix(full, cleanRoot+string(os.PathSeparator)) {
		return "", ErrOutsideRoot
	}

	return full, nil
}

// NormalizeFolder validates a folder name and returns its canonical form.
// An empty folder means the client root.
func (r *StorageService) NormalizeFolder(folder string) (string, error) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return "", nil
	}

	// A leading or trailing slash, a parent reference or a backslash are all
	// signs of an attempt to leave the client root.
	if strings.HasPrefix(folder, "/") || strings.HasSuffix(folder, "/") {
		return "", ErrInvalidFolder
	}
	if strings.Contains(folder, "..") || strings.Contains(folder, "\\") {
		return "", ErrInvalidFolder
	}

	if !folderPattern.MatchString(folder) {
		return "", ErrInvalidFolder
	}

	return folder, nil
}

// MaxFileSize returns the upload limit that applies to a client.
func (r *StorageService) MaxFileSize(client *models.Client) int64 {
	if client != nil && client.MaxFileSize != nil && *client.MaxFileSize > 0 {
		return *client.MaxFileSize
	}

	limit := facades.Config().GetInt("storage.max_file_size")
	if limit <= 0 {
		return 52428800
	}

	return int64(limit)
}

// MimeAllowed reports whether a detected MIME type passes the client's filter.
// An empty filter allows everything; entries may be exact or "type/*".
func (r *StorageService) MimeAllowed(detected string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}

	detected = strings.ToLower(strings.TrimSpace(detected))
	for _, pattern := range allowed {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" || pattern == "*" || pattern == "*/*" || pattern == detected {
			return true
		}
		if strings.HasSuffix(pattern, "/*") && strings.HasPrefix(detected, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}

	return false
}

// UsedBytes is the total size of the files a client currently stores.
func (r *StorageService) UsedBytes(clientID uint) (int64, error) {
	var used *int64
	if err := facades.Orm().Query().Model(&models.File{}).Where("client_id", clientID).Sum("size", &used); err != nil {
		return 0, err
	}
	if used == nil {
		return 0, nil
	}

	return *used, nil
}

// Inspect reads the beginning of an uploaded file to detect its real MIME type
// without trusting the Content-Type sent by the client.
func (r *StorageService) Inspect(file contractsfilesystem.File, originalName string) (Candidate, error) {
	candidate := Candidate{File: file, OriginalName: originalName}

	source, err := os.Open(file.File())
	if err != nil {
		return candidate, err
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return candidate, err
	}
	candidate.Size = info.Size()

	head := make([]byte, 512)
	read, err := io.ReadFull(source, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return candidate, err
	}
	candidate.Mime = trimMimeParameters(http.DetectContentType(head[:read]))

	// The extension is recorded separately, never folded into the detected
	// type: doing so would let "virus.exe" renamed to "photo.png" pass an
	// image/* allow-list.
	candidate.ExtensionMime = trimMimeParameters(mime.TypeByExtension(strings.ToLower(filepath.Ext(originalName))))

	return candidate, nil
}

// trimMimeParameters drops the "; charset=..." tail of a media type.
func trimMimeParameters(value string) string {
	if index := strings.Index(value, ";"); index > 0 {
		return strings.TrimSpace(value[:index])
	}

	return strings.TrimSpace(value)
}

// Write copies an inspected upload onto the right disk and returns the record
// that still has to be persisted, together with the absolute path it landed
// on so the caller can unlink it if the surrounding transaction rolls back.
func (r *StorageService) Write(clientID uint, folder, visibility, storedMime string, candidate Candidate) (*models.File, string, error) {
	if visibility != models.VisibilityPrivate {
		visibility = models.VisibilityPublic
	}

	disk := "public"
	if visibility == models.VisibilityPrivate {
		disk = "private"
	}

	extension := strings.ToLower(strings.TrimPrefix(filepath.Ext(candidate.OriginalName), "."))
	if extension == "" {
		if guesses, _ := mime.ExtensionsByType(candidate.Mime); len(guesses) > 0 {
			extension = strings.TrimPrefix(guesses[0], ".")
		}
	}
	if len(extension) > 20 {
		extension = extension[:20]
	}

	name := uuid.NewString()
	if extension != "" {
		name += "." + extension
	}

	now := time.Now().UTC()
	segments := []string{strconv.FormatUint(uint64(clientID), 10)}
	if folder != "" {
		segments = append(segments, folder)
	}
	segments = append(segments, now.Format("2006"), now.Format("01"), name)
	relative := strings.Join(segments, "/")

	target, err := r.ResolvePath(disk, relative)
	if err != nil {
		return nil, "", err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, "", err
	}

	source, err := os.Open(candidate.File.File())
	if err != nil {
		return nil, "", err
	}
	defer source.Close()

	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, "", err
	}

	// Hash and write in a single pass so large uploads never sit in memory.
	digest := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(destination, digest), source)
	closeErr := destination.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return nil, "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return nil, "", closeErr
	}

	record := &models.File{
		ClientID:     clientID,
		Folder:       folder,
		Name:         name,
		OriginalName: candidate.OriginalName,
		Path:         relative,
		Mime:         storedMime,
		Extension:    extension,
		Size:         written,
		Sha256:       hex.EncodeToString(digest.Sum(nil)),
		Visibility:   visibility,
	}

	return record, target, nil
}

// AbsolutePath returns the on-disk location of a stored file.
func (r *StorageService) AbsolutePath(file *models.File) (string, error) {
	return r.ResolvePath(file.Disk(), file.Path)
}

// Delete removes a file from the database first and only then from disk. If
// the unlink fails the row is already gone, so the next storage:sync run sees
// an orphan it can clean up; the other order would leave a row pointing at
// nothing whenever the delete failed after the unlink.
func (r *StorageService) Delete(file *models.File) error {
	target, err := r.AbsolutePath(file)
	if err != nil {
		return err
	}

	if err := facades.Orm().Transaction(func(tx orm.Query) error {
		_, deleteErr := tx.Delete(file)

		return deleteErr
	}); err != nil {
		return err
	}

	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		facades.Log().Warning("failed to unlink a deleted file: " + target + ": " + err.Error())
	}

	return nil
}

// FolderStat is one row of the folder listing endpoints.
type FolderStat struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	FilesCount int64  `json:"files_count"`
	Size       int64  `json:"size"`
}

type folderRow struct {
	Folder     string `gorm:"column:folder"`
	FilesCount int64  `gorm:"column:files_count"`
	Size       int64  `gorm:"column:size"`
}

// Folders lists the direct sub folders of parent for a client, with the number
// of files and total size stored underneath each of them.
func (r *StorageService) Folders(clientID uint, parent string) ([]FolderStat, error) {
	var rows []folderRow
	err := facades.Orm().Query().Model(&models.File{}).
		Where("client_id", clientID).
		Select("folder, count(*) as files_count, coalesce(sum(size), 0) as size").
		GroupBy("folder").
		Get(&rows)
	if err != nil {
		return nil, err
	}

	prefix := ""
	depth := 0
	if parent != "" {
		prefix = parent + "/"
		depth = len(strings.Split(parent, "/"))
	}

	grouped := map[string]*FolderStat{}
	order := []string{}

	for _, row := range rows {
		if row.Folder == "" || (prefix != "" && !strings.HasPrefix(row.Folder, prefix)) {
			continue
		}

		segments := strings.Split(row.Folder, "/")
		if len(segments) <= depth {
			continue
		}

		child := strings.Join(segments[:depth+1], "/")
		stat, ok := grouped[child]
		if !ok {
			stat = &FolderStat{Name: segments[depth], Path: child}
			grouped[child] = stat
			order = append(order, child)
		}
		stat.FilesCount += row.FilesCount
		stat.Size += row.Size
	}

	result := make([]FolderStat, 0, len(order))
	for _, key := range order {
		result = append(result, *grouped[key])
	}

	return result, nil
}

// PublicURL returns the browser reachable URL of a public file, or nil when
// the file is private.
func (r *StorageService) PublicURL(file *models.File) *string {
	if file.Visibility != models.VisibilityPublic {
		return nil
	}

	url := fmt.Sprintf("%s/storage/%s", strings.TrimRight(facades.Config().GetString("http.url"), "/"), file.Path)

	return &url
}

// SanitizeFileName strips anything that could break a Content-Disposition
// header or escape a directory.
func SanitizeFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(char rune) rune {
		if char < 32 || char == '"' || char == '/' || char == 127 {
			return '_'
		}

		return char
	}, name)

	if name == "" || name == "." || name == ".." {
		return "download"
	}

	return name
}

// EscapeLike neutralises the wildcards of a LIKE/ILIKE pattern so a user
// supplied search term is matched literally. Postgres treats a backslash as
// the default escape character, so no ESCAPE clause is needed.
func EscapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

	return replacer.Replace(value)
}

// ContentDisposition builds an attachment header that carries both the legacy
// ASCII file name and the RFC 8187 encoded one, so non ASCII names survive.
func ContentDisposition(name string) string {
	ascii := SanitizeFileName(name)
	ascii = strings.Map(func(char rune) rune {
		if char > 126 {
			return '_'
		}

		return char
	}, ascii)
	if ascii == "" {
		ascii = "download"
	}

	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", ascii, encodeAttrChar(SanitizeFileName(name)))
}

// encodeAttrChar percent-encodes a string for the ext-value of RFC 8187.
//
// url.PathEscape is not a substitute: it leaves $ & + , / : ; = ? @ and a few
// others untouched, and several of those are separators inside a
// Content-Disposition header, so a file name containing one of them would
// truncate or corrupt the parameter. RFC 8187 allows exactly attr-char, that
// is ALPHA / DIGIT / ! # $ & + - . ^ _ ` | ~, and everything else has to be
// written as %XX over the UTF-8 bytes.
func encodeAttrChar(value string) string {
	const attrChars = "!#$&+-.^_`|~"

	var out strings.Builder
	for index := 0; index < len(value); index++ {
		char := value[index]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.IndexByte(attrChars, char) >= 0 {
			out.WriteByte(char)

			continue
		}

		fmt.Fprintf(&out, "%%%02X", char)
	}

	return out.String()
}
