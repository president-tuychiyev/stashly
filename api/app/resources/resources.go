// Package resources turns database models into the JSON shapes described by
// the API contract.
package resources

import (
	"fmt"
	"strings"
	"time"

	"github.com/goravel/framework/support/carbon"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// BaseURL is the public address of this service, without a trailing slash.
func BaseURL() string {
	return strings.TrimRight(facades.Config().GetString("http.url"), "/")
}

// Time formats a timestamp as RFC3339 in UTC, or returns nil when there is
// nothing to format.
func Time(value *carbon.DateTime) *string {
	if value == nil || value.IsZero() {
		return nil
	}

	formatted := value.StdTime().UTC().Format(time.RFC3339)

	return &formatted
}

// File renders a stored file. Private files carry a null url because they are
// only reachable through the download endpoints.
func File(file *models.File) map[string]any {
	url := services.NewStorageService().PublicURL(file)

	out := map[string]any{
		"id":            file.ID,
		"client_id":     file.ClientID,
		"folder":        file.Folder,
		"name":          file.Name,
		"original_name": file.OriginalName,
		"path":          file.Path,
		"url":           url,
		"mime":          file.Mime,
		"extension":     file.Extension,
		"size":          file.Size,
		"sha256":        file.Sha256,
		"visibility":    file.Visibility,
		"created_at":    Time(file.CreatedAt),
	}

	if file.Client != nil {
		out["client"] = map[string]any{"id": file.Client.ID, "name": file.Client.Name}
	}

	return out
}

// Files renders a list of files.
func Files(files []models.File) []map[string]any {
	out := make([]map[string]any, 0, len(files))
	for index := range files {
		out = append(out, File(&files[index]))
	}

	return out
}

// Archive renders an archive. downloadPrefix is either "/admin/archives" or
// "/api/archives" depending on which surface is answering.
func Archive(archive *models.Archive, downloadPrefix string) map[string]any {
	var url *string
	if archive.Status == models.ArchiveStatusDone && archive.Path != nil {
		value := fmt.Sprintf("%s%s/%d/download", BaseURL(), downloadPrefix, archive.ID)
		url = &value
	}

	folders := archive.Folders
	if folders == nil {
		folders = []string{}
	}

	out := map[string]any{
		"id":        archive.ID,
		"client_id": archive.ClientID,
		"created_by": map[string]any{
			"type": archive.CreatedByType,
			"id":   archive.CreatedByID,
		},
		"folders":      folders,
		"status":       archive.Status,
		"progress":     archive.Progress,
		"files_count":  archive.FilesCount,
		"size":         archive.Size,
		"path":         archive.Path,
		"url":          url,
		"error":        archive.Error,
		"callback_url": archive.CallbackURL,
		"expires_at":   Time(archive.ExpiresAt),
		"created_at":   Time(archive.CreatedAt),
		"finished_at":  Time(archive.FinishedAt),
	}

	if archive.Client != nil {
		out["client"] = map[string]any{"id": archive.Client.ID, "name": archive.Client.Name}
	}

	return out
}

// Archives renders a list of archives.
func Archives(archives []models.Archive, downloadPrefix string) []map[string]any {
	out := make([]map[string]any, 0, len(archives))
	for index := range archives {
		out = append(out, Archive(&archives[index], downloadPrefix))
	}

	return out
}

// Client renders a client together with its computed usage figures.
func Client(client *models.Client, usedBytes, filesCount int64) map[string]any {
	allowed := client.AllowedMimes
	if allowed == nil {
		allowed = []string{}
	}

	var owner any
	if client.Owner != nil {
		owner = map[string]any{
			"id":    client.Owner.ID,
			"name":  client.Owner.Name,
			"email": client.Owner.Email,
		}
	}

	return map[string]any{
		"owner_id":      client.OwnerID,
		"owner":         owner,
		"id":            client.ID,
		"name":          client.Name,
		"username":      client.Username,
		"status":        client.Status,
		"quota_bytes":   client.QuotaBytes,
		"used_bytes":    usedBytes,
		"files_count":   filesCount,
		"allowed_mimes": allowed,
		"max_file_size": client.MaxFileSize,
		"creator_id":    client.CreatorID,
		"updater_id":    client.UpdaterID,
		"created_at":    Time(client.CreatedAt),
		"updated_at":    Time(client.UpdatedAt),
	}
}

// User renders an admin panel account. clientsCount is how many clients the
// user owns; pass what services.UserService.ClientsCount returned.
func User(user *models.User, clientsCount int64) map[string]any {
	out := map[string]any{
		"id":            user.ID,
		"name":          user.Name,
		"email":         user.Email,
		"avatar_src":    user.AvatarSrc,
		"status":        user.Status,
		"clients_count": clientsCount,
		"last_login_at": Time(user.LastLoginAt),
		"created_at":    Time(user.CreatedAt),
	}

	if user.Role != nil {
		permissions := user.Role.Permissions
		if permissions == nil {
			permissions = []string{}
		}
		out["role"] = map[string]any{
			"id":          user.Role.ID,
			"name":        user.Role.Name,
			"slug":        user.Role.Slug,
			"permissions": permissions,
		}
	} else {
		out["role"] = nil
	}

	return out
}

// AuditLog renders one audit trail entry.
func AuditLog(log *models.AuditLog) map[string]any {
	details := log.Details
	if details == nil {
		details = map[string]any{}
	}

	return map[string]any{
		"id":        log.ID,
		"client_id": log.ClientID,
		"actor": map[string]any{
			"type": log.ActorType,
			"id":   log.ActorID,
			"name": log.ActorName,
		},
		"action":       log.Action,
		"subject_type": log.SubjectType,
		"subject_id":   log.SubjectID,
		"details":      details,
		"ip":           log.IP,
		"created_at":   Time(log.CreatedAt),
	}
}

// AuditLogs renders a list of audit trail entries.
func AuditLogs(logs []models.AuditLog) []map[string]any {
	out := make([]map[string]any, 0, len(logs))
	for index := range logs {
		out = append(out, AuditLog(&logs[index]))
	}

	return out
}

// Device renders one registered device of a client.
func Device(device *models.Device) map[string]any {
	return map[string]any{
		"id":           device.ID,
		"uid":          device.UID,
		"platform":     device.Platform,
		"app_version":  device.AppVersion,
		"ip":           device.IP,
		"last_seen_at": Time(device.LastSeenAt),
		"client_id":    device.ClientID,
		"created_at":   Time(device.CreatedAt),
	}
}

// Devices renders a list of devices.
func Devices(devices []models.Device) []map[string]any {
	out := make([]map[string]any, 0, len(devices))
	for index := range devices {
		out = append(out, Device(&devices[index]))
	}

	return out
}

// Users renders a list of admin accounts.
func Users(users []models.User, clientsCount map[uint]int64) []map[string]any {
	out := make([]map[string]any, 0, len(users))
	for index := range users {
		out = append(out, User(&users[index], clientsCount[users[index].ID]))
	}

	return out
}
