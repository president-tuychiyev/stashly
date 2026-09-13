package config

import (
	"os"

	"github.com/goravel/framework/support/path"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

func init() {
	config := facades.Config()

	// Make sure every disk root exists before the application starts serving.
	for _, dir := range []string{"app/public", "app/private", "app/archives"} {
		_ = os.MkdirAll(path.Storage(dir), 0o755)
	}

	config.Add("filesystems", map[string]any{
		// Default Filesystem Disk
		"default": "public",

		// Filesystem Disks
		//
		// public   - files reachable through the /storage static route.
		// private  - files only reachable through the download endpoints.
		// archives - generated zip files.
		"disks": map[string]any{
			"public": map[string]any{
				"driver": "local",
				"root":   path.Storage("app/public"),
				"url":    config.Env("APP_URL", "").(string) + "/storage",
			},
			"private": map[string]any{
				"driver": "local",
				"root":   path.Storage("app/private"),
			},
			"archives": map[string]any{
				"driver": "local",
				"root":   path.Storage("app/archives"),
			},
		},
	})
}
