package services

import (
	"strconv"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

// The cache keys shared between the middleware that fills them and the
// services that have to drop them again live here, so a rename cannot leave a
// stale entry behind.

// ClientCacheKey holds the client row the api_check middleware authenticates
// against.
func ClientCacheKey(username string) string {
	return "client:" + username
}

// ClientAuthCacheKey holds the keyed digest that lets api_check skip bcrypt
// for a password it has already verified.
func ClientAuthCacheKey(username string) string {
	return "client_auth:" + username
}

// RoleCacheKey holds the role the can_user middleware checks permissions
// against.
func RoleCacheKey(roleID uint) string {
	return "role:" + strconv.FormatUint(uint64(roleID), 10)
}

// ForgetRole drops the cached copy of a role so that a permission change takes
// effect immediately instead of after the 10 minute TTL.
//
// Call it from wherever a role is written: the role admin endpoints (once they
// exist), the role seeder, and any console command that edits permissions or
// deactivates a role. Nothing caches a role anywhere else.
func ForgetRole(roleID uint) {
	facades.Cache().Forget(RoleCacheKey(roleID))
}
