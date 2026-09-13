package bootstrap

import (
	"strings"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// CheckSecrets refuses to run without the secrets the security of the whole
// service rests on. It is fatal on purpose: a booted process with an empty
// application key would keep serving, and every one time code it stores would
// be forgeable by anybody who knows the address and the purpose, because the
// HMAC would be keyed with nothing.
func CheckSecrets() {
	if strings.TrimSpace(facades.Config().GetString("app.key")) == "" {
		facades.Log().Fatal(
			"APP_KEY is empty: the one time code digests and the encrypter are keyed with it. " +
				"Set APP_KEY to a 32 character random string (php-style: `go run . artisan key:generate`, " +
				"or any 32 byte secret) and start again")
	}

	// Touching the derived key here means a broken configuration is reported
	// at boot rather than on the first invite.
	_ = services.OtpKey()
}
