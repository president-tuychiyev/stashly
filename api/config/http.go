package config

import (
	"github.com/goravel/framework/contracts/route"
	ginfacades "github.com/goravel/gin/facades"
	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/server"
)

func init() {
	config := facades.Config()

	config.Add("http", map[string]any{
		"default": "gin",
		// HTTP Drivers
		"drivers": map[string]any{
			"gin": map[string]any{
				// Gin uses body_limit (kilobytes) as MaxMultipartMemory, i.e. the
				// in-memory buffer of the multipart parser, NOT a cap on the request
				// size: anything larger simply spills to a temp file. A modest 4 MB
				// buffer keeps normal uploads in RAM without letting a request pin an
				// unbounded amount of it; anything above it lands in a temp file that
				// UploadService removes again. Note that goravel/gin's own
				// getHttpBody() parses with a hard coded 32 MB buffer of its own, so
				// this value only bounds the parses gin itself starts. The real cap
				// on the number of bytes that can reach either parser is enforced
				// before the body is read at all by
				// server.WrapBodyLimit (http.MaxBytesReader
				// installed above the gin engine), then per client by the
				// limit_upload_size middleware plus a per part check against the
				// multipart header size in UploadService.
				"body_limit":   4096,
				"header_limit": 4096,
				// The gin route is wrapped so that the hard body cap runs before
				// Goravel's global middleware, which parses the multipart form.
				"route": func() (route.Route, error) {
					return server.WrapBodyLimit(ginfacades.Route("gin")), nil
				},
			},
		},
		// HTTP URL
		"url": config.Env("APP_URL", "http://localhost"),
		// HTTP Host
		"host": config.Env("APP_HOST", "127.0.0.1"),
		// HTTP Port
		"port": config.Env("APP_PORT", "3000"),
		// HTTP Timeout, default is 3 seconds
		"request_timeout": 300,
		// HTTPS Configuration
		"tls": map[string]any{
			// HTTPS Host
			"host": config.Env("APP_HOST", "127.0.0.1"),
			// HTTPS Port
			"port": config.Env("APP_PORT", "3000"),
			// SSL Certificate, you can put the certificate in /public folder
			"ssl": map[string]any{
				// ca.pem
				"cert": "",
				// ca.key
				"key": "",
			},
		},
		// Default Client Name
		//
		// This determines which client is used when you call facades.Http() or
		// facades.Http().Client() without passing a specific name.
		"default_client": config.Env("HTTP_CLIENT_DEFAULT", "default"),
		// Client Configurations
		//
		// Here you may define multiple independent client configurations.
		// For example, you might have a "github" client with a specific base URL
		// and a "stripe" client with a longer timeout.
		"clients": map[string]any{
			"default": map[string]any{
				// The base URL for the client. All requests made using this client
				// will be relative to this URL.
				"base_url": config.Env("HTTP_CLIENT_BASE_URL", ""),
				// The maximum amount of time a request can take, including connection
				// establishment, redirects, and reading the response body.
				"timeout": config.Env("HTTP_CLIENT_TIMEOUT", "30s"),
				// The maximum number of idle (keep-alive) connections to keep across
				// ALL hosts. Increasing this helps reuse TCP connections.
				"max_idle_conns": config.Env("HTTP_CLIENT_MAX_IDLE_CONNS", 100),
				// The maximum number of idle (keep-alive) connections to keep PER host.
				// By default, Go sets this to 2, which is often a bottleneck.
				// Increase this value for high-throughput applications.
				"max_idle_conns_per_host": config.Env("HTTP_CLIENT_MAX_IDLE_CONNS_PER_HOST", 2),
				// The maximum total number of connections (active + idle) allowed per host.
				// A value of 0 means no limit.
				"max_conns_per_host": config.Env("HTTP_CLIENT_MAX_CONN_PER_HOST", 0),
				// The maximum amount of time an idle (keep-alive) connection will remain
				// in the pool before closing itself.
				"idle_conn_timeout": config.Env("HTTP_CLIENT_IDLE_CONN_TIMEOUT", "90s"),
			},
		},
	})
}
