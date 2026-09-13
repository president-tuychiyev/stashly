package config

import (
	"strings"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

func init() {
	config := facades.Config()
	config.Add("cors", map[string]any{
		// Cross-Origin Resource Sharing (CORS) Configuration
		//
		// Here you may configure your settings for cross-origin resource sharing
		// or "CORS". This determines what cross-origin operations may execute
		// in web browsers. You are free to adjust these settings as needed.
		//
		// To learn more: https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
		"paths":           []string{"*"},
		"allowed_methods": []string{"*"},
		// Comma separated list in CORS_ALLOWED_ORIGINS, "*" allows everything.
		"allowed_origins":      allowedOrigins(config.Env("CORS_ALLOWED_ORIGINS", "*")),
		"allowed_headers":      []string{"*"},
		"exposed_headers":      []string{"Content-Disposition"},
		"max_age":              0,
		"supports_credentials": false,
	})
}

// allowedOrigins splits the comma separated CORS_ALLOWED_ORIGINS value and
// falls back to "*" when nothing usable is configured.
func allowedOrigins(raw any) []string {
	value, _ := raw.(string)

	origins := make([]string, 0, 4)
	for _, origin := range strings.Split(value, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}

	if len(origins) == 0 {
		return []string{"*"}
	}

	return origins
}
