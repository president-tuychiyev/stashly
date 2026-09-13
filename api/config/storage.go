package config

import (
	"github.com/president-tuychiyev/stashly/api/app/facades"
)

// DefaultMaxFileSize is the fallback upload limit (50 MB) when MAX_FILE_SIZE is not set.
const DefaultMaxFileSize = 52428800

func init() {
	config := facades.Config()
	config.Add("storage", map[string]any{
		// Largest single upload accepted, in bytes. A client may lower this
		// through its own max_file_size column.
		"max_file_size": config.Env("MAX_FILE_SIZE", DefaultMaxFileSize),

		// How long a finished archive stays downloadable, in days.
		"archive_ttl_days": config.Env("ARCHIVE_TTL_DAYS", 5),

		// Uploads allowed per minute and per client.
		"upload_rate_per_minute": config.Env("UPLOAD_RATE_PER_MINUTE", 60),

		// Maximum number of files accepted in one multiple upload request.
		"max_files_per_request": 20,
	})
}
