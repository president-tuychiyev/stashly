// Package admin holds the controllers of the /admin surface used by the web
// application. Every route is protected by the JWT guard and can_user.
package admin

import (
	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// AuthUser returns the admin user put in the context by can_user.
func AuthUser(ctx http.Context) *models.User {
	if user, ok := ctx.Value("auth_user").(*models.User); ok {
		return user
	}

	return nil
}

// AuthUserID returns the id of the admin user, or nil when there is none.
func AuthUserID(ctx http.Context) *uint {
	if user := AuthUser(ctx); user != nil {
		id := user.ID

		return &id
	}

	return nil
}

// Actor builds the audit actor for the current admin user.
func Actor(ctx http.Context) services.Actor {
	return services.UserActor(AuthUser(ctx))
}

// MailFailure is the answer every endpoint gives when a one time code could
// not be delivered. The reason, which names hosts and credentials, goes to the
// log; the caller gets one flat sentence. Answering 204 here instead would
// tell the user a code is on its way that nobody ever sent.
func MailFailure(ctx http.Context, err error) http.Response {
	facades.Log().With(map[string]any{
		"path": ctx.Request().Path(),
		"ip":   ctx.Request().Ip(),
	}).Error("could not send an otp email: " + err.Error())

	return responses.Error(ctx, http.StatusInternalServerError, "could not send email")
}

// claimResendGap enforces the resend gap for the public endpoints before they
// know whether the address belongs to anybody.
//
// Without it the gap is only enforced by the otp_codes table, which exists for
// known addresses only: an unknown address would answer 204 forever while a
// known one starts answering 429 after the first call, and the difference is
// an account enumeration oracle. The bucket is per (IP, address) so one caller
// cannot walk a list, and a shared NAT cannot lock out a whole office for
// somebody else's address.
//
// It reports false when the bucket is still warm, which the caller answers 429.
func claimResendGap(ctx http.Context, email string) bool {
	key := "otp_gap:" + ctx.Request().Ip() + ":" + services.NormalizeEmail(email)

	if facades.Cache().Has(key) {
		return false
	}

	if err := facades.Cache().Put(key, true, services.OtpResendGap); err != nil {
		// A cache that cannot be written must not turn into an open relay.
		facades.Log().Warning("could not claim the otp resend gap: " + err.Error())

		return false
	}

	return true
}
