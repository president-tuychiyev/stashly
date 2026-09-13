package admin

import (
	"errors"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/support/carbon"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/requests"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// ProfileController serves the endpoints a user has over its own account,
// whatever its role is.
type ProfileController struct {
	users *services.UserService
	otp   *services.OtpService
	audit *services.AuditService
}

func NewProfileController() *ProfileController {
	return &ProfileController{
		users: services.NewUserService(),
		otp:   services.NewOtpService(),
		audit: services.NewAuditService(),
	}
}

// Show returns the authenticated user.
func (r *ProfileController) Show(ctx http.Context) http.Response {
	user := AuthUser(ctx)
	if user == nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	r.users.LoadRole(user)

	return responses.Data(ctx, http.StatusOK, resources.User(user, r.users.ClientsCount(user.ID)))
}

// Update changes the display name.
func (r *ProfileController) Update(ctx http.Context) http.Response {
	user := AuthUser(ctx)
	if user == nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	var request requests.UpdateProfileRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	user.Name = request.Name
	if err := facades.Orm().Query().Save(user); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	r.users.LoadRole(user)

	id := user.ID
	r.audit.Log(Actor(ctx), "user.update", "user", &id, map[string]any{"self": true}, ctx.Request().Ip())

	return responses.Data(ctx, http.StatusOK, resources.User(user, r.users.ClientsCount(user.ID)))
}

// UpdatePassword replaces the password after checking the current one.
func (r *ProfileController) UpdatePassword(ctx http.Context) http.Response {
	user := AuthUser(ctx)
	if user == nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	var request requests.ChangePasswordRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	if user.Password == nil || !facades.Hash().Check(request.CurrentPassword, *user.Password) {
		return responses.InvalidField(ctx, "current_password", "the current password is wrong")
	}

	if err := services.ValidatePassword(request.Password, request.PasswordConfirmation); err != nil {
		return responses.InvalidField(ctx, "password", err.Error())
	}

	if err := r.users.SetPassword(user, request.Password); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	id := user.ID
	r.audit.Log(Actor(ctx), "user.password_change", "user", &id,
		map[string]any{"via": "profile"}, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

// RequestEmailChange mails a confirmation code to the NEW address. Nothing is
// written until that code comes back, so a typo cannot lock anybody out.
func (r *ProfileController) RequestEmailChange(ctx http.Context) http.Response {
	user := AuthUser(ctx)
	if user == nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	var request requests.ChangeEmailRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	if user.Password == nil || !facades.Hash().Check(request.CurrentPassword, *user.Password) {
		return responses.InvalidField(ctx, "current_password", "the current password is wrong")
	}

	email := services.NormalizeEmail(request.Email)
	if email == user.Email {
		return responses.InvalidField(ctx, "email", "this is already your address")
	}

	taken, err := r.users.EmailTaken(email, user.ID)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}
	if taken {
		return responses.InvalidField(ctx, "email", "email is already taken")
	}

	// UserID both stores the owner in its own indexed column and, inside
	// Issue, consumes every other outstanding email change of this user: an
	// abandoned request to a third address must not stay a live path to it.
	err = r.otp.Issue(services.OtpIssueInput{
		Email:    email,
		Purpose:  models.OtpPurposeEmailChange,
		Name:     user.Name,
		NewEmail: email,
		UserID:   &user.ID,
		Meta:     map[string]any{"new_email": email},
	})
	if err != nil {
		if errors.Is(err, services.ErrOtpResendTooSoon) {
			return responses.Error(ctx, http.StatusTooManyRequests, err.Error())
		}

		return MailFailure(ctx, err)
	}

	id := user.ID
	r.audit.Log(Actor(ctx), "otp.send", "user", &id,
		map[string]any{"purpose": models.OtpPurposeEmailChange}, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

// ConfirmEmailChange applies the pending address once its code checks out.
func (r *ProfileController) ConfirmEmailChange(ctx http.Context) http.Response {
	user := AuthUser(ctx)
	if user == nil {
		return responses.Error(ctx, http.StatusUnauthorized, "Unauthenticated")
	}

	var request requests.ConfirmEmailRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	pending, ok := r.otp.PendingEmailChange(user.ID)
	if !ok {
		return responses.InvalidField(ctx, "code", services.ErrOtpInvalid.Error())
	}

	meta, err := r.otp.Verify(pending, models.OtpPurposeEmailChange, request.Code)
	if err != nil {
		return responses.InvalidField(ctx, "code", services.ErrOtpInvalid.Error())
	}

	// The lookup above found the pending address by user, but Verify consumes
	// the newest outstanding code for that address, which somebody else may
	// have asked for in the meantime. A code that does not belong to the
	// caller is simply an invalid code.
	if services.MetaUserID(meta) != user.ID {
		return responses.InvalidField(ctx, "code", services.ErrOtpInvalid.Error())
	}

	email := services.NormalizeEmail(pending)
	if value, found := meta["new_email"].(string); found && value != "" {
		email = services.NormalizeEmail(value)
	}

	// The address was free when the code was sent; somebody else may have
	// taken it in the meantime, so it is checked again before the write.
	taken, err := r.users.EmailTaken(email, user.ID)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}
	if taken {
		return responses.InvalidField(ctx, "email", "email is already taken")
	}

	previous := user.Email
	user.Email = email
	user.EmailVerifiedAt = carbon.NewDateTime(carbon.Now())
	// The login identity just changed, so the tokens issued against the old
	// one are no longer a session anybody agreed to.
	r.users.TouchCredentials(user)
	if err := facades.Orm().Query().Save(user); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	r.users.LoadRole(user)

	id := user.ID
	r.audit.Log(Actor(ctx), "user.email_change", "user", &id,
		map[string]any{"from": previous, "to": email}, ctx.Request().Ip())

	return responses.Data(ctx, http.StatusOK, resources.User(user, r.users.ClientsCount(user.ID)))
}
