package config

import (
	"github.com/president-tuychiyev/stashly/api/app/facades"
)

func init() {
	config := facades.Config()
	config.Add("mail", map[string]any{
		// Mailer
		//
		// "smtp" delivers through the SMTP server configured below, "log"
		// writes the rendered message to the application log instead. The log
		// mailer is what local development and the test suite use: nothing
		// leaves the machine and the one time codes stay readable. It is
		// refused outside APP_ENV local/testing, so a deployment that inherits
		// a stray MAIL_MAILER=log fails at boot instead of quietly dropping
		// every invite and reset code.
		//
		// Supported: "smtp" (default), "log"
		"mailer": config.Env("MAIL_MAILER", "smtp"),

		// SMTP host and port. The framework picks the transport from the port:
		// 465 wraps the whole connection in TLS, 587 upgrades with STARTTLS,
		// anything else stays plain. MAIL_ENCRYPTION below is kept so the
		// deployment can state its intent explicitly; the mailer refuses to
		// send over a port that contradicts it.
		"host": config.Env("MAIL_HOST", ""),
		"port": config.Env("MAIL_PORT", 587),

		// Supported: "tls" (STARTTLS, port 587), "ssl" (implicit TLS, port
		// 465), "none" (no transport security, development only).
		"encryption": config.Env("MAIL_ENCRYPTION", "tls"),

		"username": config.Env("MAIL_USERNAME", ""),
		"password": config.Env("MAIL_PASSWORD", ""),

		// Global "From" address used by every message this service sends.
		"from": map[string]any{
			"address": config.Env("MAIL_FROM_ADDRESS", "no-reply@stashly.local"),
			"name":    config.Env("MAIL_FROM_NAME", "Stashly"),
		},

		// Template Configuration
		//
		// The OTP messages are rendered in app/mail with the standard library
		// template packages, so nothing reads from this path. It is kept
		// because the framework's mail application builds its engine eagerly.
		"template": map[string]any{
			"default": config.Env("MAIL_TEMPLATE_ENGINE", "html"),
			"engines": map[string]any{
				"html": map[string]any{
					"driver": "html",
					"path":   config.Env("MAIL_VIEWS_PATH", "resources/views/mail"),
				},
			},
		},
	})
}
