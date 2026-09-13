package routes

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cast"

	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
	"github.com/goravel/framework/http/limit"
	frameworkmiddleware "github.com/goravel/framework/http/middleware"
	"github.com/goravel/framework/support/path"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/controllers/admin"
	"github.com/president-tuychiyev/stashly/api/app/http/controllers/api"
	"github.com/president-tuychiyev/stashly/api/app/http/middleware"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

const modulePath = "github.com/president-tuychiyev/stashly/api/"

// uploadLimiter is the name of the rate limiter guarding file uploads.
const uploadLimiter = "uploads"

// loginLimiter is the name of the rate limiter guarding the admin login.
const loginLimiter = "admin-login"

// otpLimiter guards the endpoints that mail a one time code.
const otpLimiter = "admin-otp"

// The authenticated endpoints have their own limiters, keyed by the user
// rather than by the address in the body: the address is not in the body at
// all for the confirm and refresh calls, and a signed in caller is already
// identified.
const (
	profileEmailLimiter        = "admin-profile-email"
	profileEmailConfirmLimiter = "admin-profile-email-confirm"
	refreshLimiter             = "admin-auth-refresh"
)

// Per user caps for the authenticated endpoints above. Requesting an email
// change sends mail, so it is the tightest; confirming only checks a code; and
// refresh is a session keep alive, which a busy panel does often but never
// dozens of times a minute.
const (
	profileEmailRequestsPerWindow = 5
	profileEmailWindowMinutes     = 10
	profileEmailConfirmsPerMinute = 10
	refreshesPerMinute            = 30
)

// loginAttemptsPerMinute is how many login attempts one IP may make for one
// email address before the endpoint starts answering 429.
const loginAttemptsPerMinute = 10

// otpRequestsPerAddress is how many codes one address may ask for inside
// otpWindow, and otpRequestsPerIP caps one address' worth of noise from a
// single caller per minute.
const (
	otpRequestsPerAddress = 3
	otpWindowMinutes      = 10
	otpRequestsPerIP      = 10
)

func Api() {
	// Recover() has to come first: goravel/gin rebuilds the whole gin engine
	// inside Route.init() whenever the recover callback or the global
	// middleware changes, and every route registered before that call is
	// silently thrown away with the old engine.
	facades.Route().Recover(recoverFunc)

	// Only the public disk is served statically, private files always go
	// through a download endpoint. safe_static makes sure an uploaded .html or
	// .svg cannot execute against this origin.
	facades.Route().Middleware(middleware.SafeStatic()).Static("/storage", path.Storage("app/public"))

	registerRateLimiters()
	registerClientRoutes()
	registerAdminRoutes()
}

// registerRateLimiters caps how many uploads a single client may send.
func registerRateLimiters() {
	perMinute := facades.Config().GetInt("storage.upload_rate_per_minute")
	if perMinute <= 0 {
		perMinute = 60
	}

	facades.RateLimiter().For(uploadLimiter, func(ctx contractshttp.Context) contractshttp.Limit {
		key := fmt.Sprintf("uploads:%v", ctx.Value("client_id"))

		return limit.PerMinute(perMinute).By(key).Response(func(ctx contractshttp.Context) {
			ctx.Request().AbortWithStatusJson(contractshttp.StatusTooManyRequests, contractshttp.Json{
				"message": "too many uploads, try again in a minute",
			})
		})
	})

	// Login is keyed by IP and email together: a shared NAT must not lock a
	// whole office out of the panel, and one account must not be brute forced
	// from a single address either.
	facades.RateLimiter().For(loginLimiter, func(ctx contractshttp.Context) contractshttp.Limit {
		key := fmt.Sprintf("admin-login:%s:%s", ctx.Request().Ip(), loginEmail(ctx))

		return limit.PerMinute(loginAttemptsPerMinute).By(key).Response(func(ctx contractshttp.Context) {
			ctx.Request().AbortWithStatusJson(contractshttp.StatusTooManyRequests, contractshttp.Json{
				"message": "too many attempts",
			})
		})
	})

	// The endpoints that send mail are limited twice over: a few codes per
	// address per ten minutes so one mailbox cannot be flooded, and a per
	// minute cap per IP so one caller cannot walk a list of addresses.
	facades.RateLimiter().ForWithLimits(otpLimiter, func(ctx contractshttp.Context) []contractshttp.Limit {
		respond := func(ctx contractshttp.Context) {
			ctx.Request().AbortWithStatusJson(contractshttp.StatusTooManyRequests, contractshttp.Json{
				"message": "too many attempts",
			})
		}

		return []contractshttp.Limit{
			limit.PerMinutes(otpWindowMinutes, otpRequestsPerAddress).
				By("admin-otp-email:" + loginEmail(ctx)).Response(respond),
			limit.PerMinute(otpRequestsPerIP).
				By("admin-otp-ip:" + ctx.Request().Ip()).Response(respond),
		}
	})

	// Asking for an email change mails a code to an address the caller chose,
	// so an authenticated account could otherwise use the panel as a mail
	// relay one address at a time.
	facades.RateLimiter().For(profileEmailLimiter, func(ctx contractshttp.Context) contractshttp.Limit {
		return limit.PerMinutes(profileEmailWindowMinutes, profileEmailRequestsPerWindow).
			By(authThrottleKey(ctx, profileEmailLimiter)).Response(tooManyAttempts)
	})

	// Confirming sends nothing, but it does guess at a code, and the per code
	// attempt counter should not be the only thing standing in the way.
	facades.RateLimiter().For(profileEmailConfirmLimiter, func(ctx contractshttp.Context) contractshttp.Limit {
		return limit.PerMinute(profileEmailConfirmsPerMinute).
			By(authThrottleKey(ctx, profileEmailConfirmLimiter)).Response(tooManyAttempts)
	})

	// Refresh mints a new token from an old one, so it is the cheapest way to
	// keep a stolen token alive; a normal panel needs it about once an hour.
	facades.RateLimiter().For(refreshLimiter, func(ctx contractshttp.Context) contractshttp.Limit {
		return limit.PerMinute(refreshesPerMinute).
			By(authThrottleKey(ctx, refreshLimiter)).Response(tooManyAttempts)
	})
}

// authThrottleKey buckets an authenticated caller by its user id, falling back
// to the IP for the endpoints that run before the auth middleware (refresh) or
// for a request whose token did not resolve.
func authThrottleKey(ctx contractshttp.Context, prefix string) string {
	if id, ok := ctx.Value("auth_user_id").(uint); ok && id > 0 {
		return fmt.Sprintf("%s:user:%d", prefix, id)
	}

	return prefix + ":ip:" + ctx.Request().Ip()
}

// tooManyAttempts is the answer every limiter in this file gives, so a caller
// cannot tell the buckets apart.
func tooManyAttempts(ctx contractshttp.Context) {
	ctx.Request().AbortWithStatusJson(contractshttp.StatusTooManyRequests, contractshttp.Json{
		"message": "too many attempts",
	})

	facades.RateLimiter().For(profileEmailLimiter, func(ctx contractshttp.Context) contractshttp.Limit {
		return limit.PerMinutes(profileEmailWindowMinutes, profileEmailRequestsPerWindow).
			By(authThrottleKey(ctx, profileEmailLimiter)).Response(tooManyAttempts)
	})

	facades.RateLimiter().For(profileEmailConfirmLimiter, func(ctx contractshttp.Context) contractshttp.Limit {
		return limit.PerMinute(profileEmailConfirmsPerMinute).
			By(authThrottleKey(ctx, profileEmailConfirmLimiter)).Response(tooManyAttempts)
	})

	facades.RateLimiter().For(refreshLimiter, func(ctx contractshttp.Context) contractshttp.Limit {
		return limit.PerMinute(refreshesPerMinute).
			By(authThrottleKey(ctx, refreshLimiter)).Response(tooManyAttempts)
	})
}

// loginEmail reads the address a public auth request is about, normalised the
// same way the store does, so the rate limit bucket cannot be dodged with a
// different capitalisation.
func loginEmail(ctx contractshttp.Context) string {
	return services.NormalizeEmail(cast.ToString(ctx.Request().Input("email")))
}

func registerClientRoutes() {
	files := api.NewFileController()
	folders := api.NewFolderController()
	archives := api.NewArchiveController()
	me := api.NewMeController()

	facades.Route().Middleware(middleware.ApiCheck()).Prefix("api").Group(func(router route.Router) {
		router.Middleware(frameworkmiddleware.Throttle(uploadLimiter), middleware.LimitUploadSize()).
			Post("/files", files.Store).Name("api.files.store")
		router.Get("/files", files.Index).Name("api.files.index")
		router.Get("/files/{id}", files.Show).Name("api.files.show")
		router.Get("/files/{id}/download", files.Download).Name("api.files.download")
		router.Delete("/files/{id}", files.Destroy).Name("api.files.destroy")

		router.Get("/folders", folders.Index).Name("api.folders.index")

		router.Post("/archives", archives.Store).Name("api.archives.store")
		router.Get("/archives", archives.Index).Name("api.archives.index")
		router.Get("/archives/{id}", archives.Show).Name("api.archives.show")
		router.Get("/archives/{id}/download", archives.Download).Name("api.archives.download")
		router.Delete("/archives/{id}", archives.Destroy).Name("api.archives.destroy")

		router.Get("/me", me.Show).Name("api.me.show")
	})
}

func registerAdminRoutes() {
	auth := admin.NewAuthController()
	profile := admin.NewProfileController()
	users := admin.NewUserController()
	dashboard := admin.NewDashboardController()
	clients := admin.NewClientController()
	files := admin.NewFileController()
	folders := admin.NewFolderController()
	archives := admin.NewArchiveController()
	auditLogs := admin.NewAuditLogController()
	storage := admin.NewStorageController()

	facades.Route().Prefix("admin").Group(func(router route.Router) {
		// Login and refresh are the only endpoints reachable without a valid
		// access token.
		router.Middleware(frameworkmiddleware.Throttle(loginLimiter)).
			Post("/auth/login", auth.Login).Name("admin.auth.login")

		// Activating an invited account and resetting a password both happen
		// before there is a token, so they sit next to login. The code itself
		// is the credential; the limiter keeps the mailing endpoints quiet.
		router.Middleware(frameworkmiddleware.Throttle(loginLimiter)).
			Post("/auth/verify", auth.Verify).Name("admin.auth.verify")
		router.Middleware(frameworkmiddleware.Throttle(loginLimiter)).
			Post("/auth/reset", auth.Reset).Name("admin.auth.reset")
		router.Middleware(frameworkmiddleware.Throttle(otpLimiter)).
			Post("/auth/verify/resend", auth.ResendVerification).Name("admin.auth.verify_resend")
		router.Middleware(frameworkmiddleware.Throttle(otpLimiter)).
			Post("/auth/forgot", auth.Forgot).Name("admin.auth.forgot")
		// Refresh runs without jwt_auth on purpose: its whole job is to accept a
		// token that has already expired, which the middleware rejects. The
		// controller parses the bearer token itself and tolerates exactly the
		// "expired" error, nothing else.
		router.Middleware(frameworkmiddleware.Throttle(refreshLimiter)).
			Post("/auth/refresh", auth.Refresh).Name("admin.auth.refresh")

		router.Middleware(middleware.JwtAuth(), middleware.CanUser()).Group(func(secured route.Router) {
			secured.Post("/auth/logout", auth.Logout).Name("admin.auth.logout")
			secured.Get("/auth/me", auth.Me).Name("admin.auth.me")

			secured.Get("/profile", profile.Show).Name("admin.profile.show")
			secured.Put("/profile", profile.Update).Name("admin.profile.update")
			secured.Put("/profile/password", profile.UpdatePassword).Name("admin.profile.password")
			secured.Middleware(frameworkmiddleware.Throttle(profileEmailLimiter)).
				Post("/profile/email", profile.RequestEmailChange).Name("admin.profile.email")
			secured.Middleware(frameworkmiddleware.Throttle(profileEmailConfirmLimiter)).
				Post("/profile/email/confirm", profile.ConfirmEmailChange).Name("admin.profile.email_confirm")

			secured.Get("/users", users.Index).Name("admin.users.index")
			secured.Post("/users", users.Store).Name("admin.users.store")
			secured.Get("/users/{id}", users.Show).Name("admin.users.show")
			secured.Put("/users/{id}", users.Update).Name("admin.users.update")
			secured.Post("/users/{id}/resend-otp", users.ResendOtp).Name("admin.users.resend_otp")
			secured.Delete("/users/{id}", users.Destroy).Name("admin.users.destroy")

			secured.Get("/dashboard", dashboard.Index).Name("admin.dashboard.index")

			secured.Get("/clients", clients.Index).Name("admin.clients.index")
			secured.Post("/clients", clients.Store).Name("admin.clients.store")
			secured.Get("/clients/{id}", clients.Show).Name("admin.clients.show")
			secured.Put("/clients/{id}", clients.Update).Name("admin.clients.update")
			secured.Post("/clients/{id}/reset-password", clients.ResetPassword).Name("admin.clients.reset_password")
			secured.Delete("/clients/{id}", clients.Destroy).Name("admin.clients.destroy")
			secured.Get("/clients/{id}/devices", clients.Devices).Name("admin.clients.devices")
			secured.Delete("/clients/{id}/devices/{device_id}", clients.DestroyDevice).Name("admin.clients.devices_destroy")

			secured.Get("/files", files.Index).Name("admin.files.index")
			secured.Middleware(middleware.LimitUploadSize()).
				Post("/files", files.Store).Name("admin.files.store")
			secured.Post("/files/bulk-delete", files.BulkDelete).Name("admin.files.bulk_delete")
			secured.Get("/files/{id}", files.Show).Name("admin.files.show")
			secured.Get("/files/{id}/download", files.Download).Name("admin.files.download")
			secured.Delete("/files/{id}", files.Destroy).Name("admin.files.destroy")

			secured.Get("/folders", folders.Index).Name("admin.folders.index")

			secured.Get("/archives", archives.Index).Name("admin.archives.index")
			secured.Post("/archives", archives.Store).Name("admin.archives.store")
			secured.Get("/archives/status", archives.Status).Name("admin.archives.status")
			secured.Get("/archives/{id}", archives.Show).Name("admin.archives.show")
			secured.Get("/archives/{id}/download", archives.Download).Name("admin.archives.download")
			secured.Delete("/archives/{id}", archives.Destroy).Name("admin.archives.destroy")

			secured.Get("/audit-logs", auditLogs.Index).Name("admin.audit_logs.index")

			secured.Post("/storage/sync", storage.Sync).Name("admin.storage.sync")
		})
	})
}

func recoverFunc(ctx contractshttp.Context, err any) {
	details := map[string]any{"error": fmt.Sprintf("%v", err)}

	if file, line, function, ok := panicOrigin(); ok {
		details["file"] = file
		details["line"] = line
		details["function"] = function
	}

	facades.Log().With(map[string]any{
		"method":  ctx.Request().Method(),
		"path":    ctx.Request().Path(),
		"details": details,
	}).Error(err)

	payload := contractshttp.Json{"message": "Internal server error"}
	// Stack frames and panic values are for the log, not for the caller. Only
	// an explicitly debugging deployment sees them in the response.
	if facades.Config().GetBool("app.debug") {
		payload["details"] = details
	}

	ctx.Request().AbortWithStatusJson(contractshttp.StatusInternalServerError, payload)
}

func panicOrigin() (string, int, string, bool) {
	pcs := make([]uintptr, 32)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(3, pcs)])

	for {
		frame, more := frames.Next()
		if strings.HasPrefix(frame.Function, modulePath) {
			file := strings.ReplaceAll(frame.File, "\\", "/")
			if i := strings.LastIndex(file, "/api/"); i >= 0 {
				file = file[i+1:]
			}
			return file, frame.Line, strings.TrimPrefix(frame.Function, modulePath), true
		}
		if !more {
			return "", 0, "", false
		}
	}
}
