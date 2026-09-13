package admin

import (
	"errors"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"
	goravelerrors "github.com/goravel/framework/errors"
	"github.com/goravel/framework/support/carbon"
	"github.com/spf13/cast"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/requests"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type AuthController struct {
	audit *services.AuditService
	users *services.UserService
	otp   *services.OtpService
}

func NewAuthController() *AuthController {
	return &AuthController{
		audit: services.NewAuditService(),
		users: services.NewUserService(),
		otp:   services.NewOtpService(),
	}
}

// Login exchanges an email and password for a JWT token.
func (r *AuthController) Login(ctx http.Context) http.Response {
	var request requests.LoginRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	user := r.users.FindByEmail(request.Email)
	if user == nil || user.Password == nil || !facades.Hash().Check(request.Password, *user.Password) {
		return responses.Error(ctx, http.StatusUnauthorized, "invalid credentials")
	}

	// The status is only checked after the password, so the answer cannot be
	// used to tell an existing account from a blocked one without the password.
	switch user.Status {
	case models.UserStatusPending:
		return responses.Error(ctx, http.StatusForbidden, "account not verified")
	case models.UserStatusBlocked:
		return responses.Error(ctx, http.StatusForbidden, "account blocked")
	}

	token, err := facades.Auth(ctx).Login(user)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	r.users.TouchLogin(user)
	r.users.LoadRole(user)

	id := user.ID
	r.audit.Log(services.UserActor(user), "auth.login", "user", &id, map[string]any{}, ctx.Request().Ip())

	return ctx.Response().Success().Json(http.Json{
		"token": token,
		"user":  resources.User(user, r.users.ClientsCount(user.ID)),
	})
}

// Verify activates an invited user: it checks the emailed code, sets the first
// password and hands out a token so the panel can continue straight away.
func (r *AuthController) Verify(ctx http.Context) http.Response {
	var request requests.CodePasswordRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	if err := services.ValidatePassword(request.Password, request.PasswordConfirmation); err != nil {
		return responses.InvalidField(ctx, "password", err.Error())
	}

	user := r.users.FindByEmail(request.Email)
	if user == nil || user.Status != models.UserStatusPending {
		return responses.InvalidField(ctx, "code", services.ErrOtpInvalid.Error())
	}

	if _, err := r.otp.Verify(request.Email, models.OtpPurposeVerify, request.Code); err != nil {
		return responses.InvalidField(ctx, "code", services.ErrOtpInvalid.Error())
	}

	hashed, err := facades.Hash().Make(request.Password)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	now := carbon.NewDateTime(carbon.Now())
	user.Password = &hashed
	user.Status = models.UserStatusActive
	user.EmailVerifiedAt = now
	if err := facades.Orm().Query().Save(user); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	token, err := facades.Auth(ctx).Login(user)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	r.users.TouchLogin(user)
	r.users.LoadRole(user)

	id := user.ID
	r.audit.Log(services.UserActor(user), "user.verify", "user", &id, map[string]any{}, ctx.Request().Ip())

	return ctx.Response().Success().Json(http.Json{
		"token": token,
		"user":  resources.User(user, r.users.ClientsCount(user.ID)),
	})
}

// ResendVerification mails a fresh activation code to a pending user.
//
// The answer is 204 whether or not the address exists, so the endpoint cannot
// be used to find out who has an account here. The one exception is the resend
// gap, which answers 429: without it the endpoint would be a free mail relay.
func (r *AuthController) ResendVerification(ctx http.Context) http.Response {
	var request requests.EmailOnlyRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	if !claimResendGap(ctx, request.Email) {
		return responses.Error(ctx, http.StatusTooManyRequests, services.ErrOtpResendTooSoon.Error())
	}

	user := r.users.FindByEmail(request.Email)
	if user == nil || user.Status != models.UserStatusPending {
		return responses.NoContent(ctx)
	}

	return r.issue(ctx, services.OtpIssueInput{
		Email:   user.Email,
		Purpose: models.OtpPurposeVerify,
		Name:    user.Name,
		UserID:  &user.ID,
	}, user)
}

// Forgot mails a password reset code.
func (r *AuthController) Forgot(ctx http.Context) http.Response {
	var request requests.EmailOnlyRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	if !claimResendGap(ctx, request.Email) {
		return responses.Error(ctx, http.StatusTooManyRequests, services.ErrOtpResendTooSoon.Error())
	}

	user := r.users.FindByEmail(request.Email)
	if user == nil || user.Status != models.UserStatusActive {
		return responses.NoContent(ctx)
	}

	return r.issue(ctx, services.OtpIssueInput{
		Email:   user.Email,
		Purpose: models.OtpPurposePasswordReset,
		Name:    user.Name,
		UserID:  &user.ID,
	}, user)
}

// Reset sets a new password from a reset code.
func (r *AuthController) Reset(ctx http.Context) http.Response {
	var request requests.CodePasswordRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	if err := services.ValidatePassword(request.Password, request.PasswordConfirmation); err != nil {
		return responses.InvalidField(ctx, "password", err.Error())
	}

	user := r.users.FindByEmail(request.Email)
	if user == nil || user.Status != models.UserStatusActive {
		return responses.InvalidField(ctx, "code", services.ErrOtpInvalid.Error())
	}

	if _, err := r.otp.Verify(request.Email, models.OtpPurposePasswordReset, request.Code); err != nil {
		return responses.InvalidField(ctx, "code", services.ErrOtpInvalid.Error())
	}

	if err := r.users.SetPassword(user, request.Password); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	id := user.ID
	r.audit.Log(services.UserActor(user), "user.password_change", "user", &id,
		map[string]any{"via": "reset"}, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

// issue sends a code and maps the resend gap onto 429.
func (r *AuthController) issue(ctx http.Context, input services.OtpIssueInput, user *models.User) http.Response {
	if err := r.otp.Issue(input); err != nil {
		if errors.Is(err, services.ErrOtpResendTooSoon) {
			return responses.Error(ctx, http.StatusTooManyRequests, err.Error())
		}

		return MailFailure(ctx, err)
	}

	id := user.ID
	r.audit.Log(services.UserActor(user), "otp.send", "user", &id,
		map[string]any{"purpose": input.Purpose}, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

// Refresh issues a fresh token for the current session.
//
// This is the one endpoint that must accept an expired access token, so it
// parses the bearer token itself instead of going through the jwt_auth
// middleware. goravel's Parse() reports an expired token with
// errors.AuthTokenExpired while still filling in the claims, which is exactly
// what Refresh() needs; every other parse error is a hard 401. Refresh()
// itself enforces jwt.refresh_ttl on top of the claims' expiry and answers
// errors.AuthRefreshTimeExceeded once that window is over, so a token that is
// expired beyond the refresh TTL is rejected here as well.
func (r *AuthController) Refresh(ctx http.Context) http.Response {
	token := strings.TrimSpace(ctx.Request().Header("Authorization"))
	if token == "" {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	payload, err := facades.Auth(ctx).Parse(token)
	if err != nil && !errors.Is(err, goravelerrors.AuthTokenExpired) {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}
	if payload == nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	// Refresh() only looks at the claims, so without this an account that was
	// blocked or deleted after signing in could keep minting fresh tokens for
	// as long as the refresh window lasts.
	var user models.User
	if err := facades.Orm().Query().Where("id", cast.ToUint(payload.Key)).First(&user); err != nil || user.ID == 0 {
		return responses.Error(ctx, http.StatusForbidden, "Forbidden")
	}
	if user.Status != models.UserStatusActive {
		return responses.Error(ctx, http.StatusForbidden, "Forbidden")
	}
	// A token handed out before the password, the address or the block changed
	// is not a session that may be extended.
	if user.CredentialsChangedAt != nil &&
		payload.IssuedAt.Before(user.CredentialsChangedAt.StdTime().Truncate(time.Second)) {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	fresh, err := facades.Auth(ctx).Refresh()
	if err != nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	return ctx.Response().Success().Json(http.Json{"token": fresh})
}

// Logout invalidates the current token.
func (r *AuthController) Logout(ctx http.Context) http.Response {
	if err := facades.Auth(ctx).Logout(); err != nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	return responses.NoContent(ctx)
}

// Me returns the authenticated admin user.
func (r *AuthController) Me(ctx http.Context) http.Response {
	user := AuthUser(ctx)
	if user == nil {
		var loaded models.User
		if err := facades.Auth(ctx).User(&loaded); err != nil || loaded.ID == 0 {
			return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
		}
		user = &loaded
	}

	r.users.LoadRole(user)

	return responses.Data(ctx, http.StatusOK, resources.User(user, r.users.ClientsCount(user.ID)))
}
