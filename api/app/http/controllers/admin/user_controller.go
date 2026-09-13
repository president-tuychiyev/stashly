package admin

import (
	"errors"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/requests"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// lastSuperAdminMessage is the refusal every path that would leave the panel
// without a super admin answers with.
const lastSuperAdminMessage = "at least one active super administrator is required"

// UserController manages the admin panel accounts. Only a super admin reaches
// it; can_user refuses every admin.users.* route for any other role.
type UserController struct {
	users *services.UserService
	otp   *services.OtpService
	audit *services.AuditService
}

func NewUserController() *UserController {
	return &UserController{
		users: services.NewUserService(),
		otp:   services.NewOtpService(),
		audit: services.NewAuditService(),
	}
}

// Index lists the panel accounts.
func (r *UserController) Index(ctx http.Context) http.Response {
	pagination := responses.ReadPagination(ctx)

	query := facades.Orm().Query().Model(&models.User{}).With("Role")
	if search := ctx.Request().Query("search"); search != "" {
		pattern := "%" + services.EscapeLike(search) + "%"
		query = query.Where("(name ILIKE ? OR email ILIKE ?)", pattern, pattern)
	}
	if status := ctx.Request().Query("status"); status != "" {
		query = query.Where("status", status)
	}
	if slug := ctx.Request().Query("role"); slug != "" {
		role, err := r.users.RoleBySlug(slug)
		if err != nil {
			// An unknown role filters everything out instead of erroring.
			query = query.Where("role_id", 0)
		} else {
			query = query.Where("role_id", role.ID)
		}
	}
	query = services.ApplySort(query, ctx.Request().Query("sort"),
		[]string{"created_at", "name", "email", "id", "last_login_at"}, "created_at")

	var users []models.User
	var total int64
	if err := query.Paginate(pagination.Page, pagination.PerPage, &users, &total); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	counts := map[uint]int64{}
	for index := range users {
		counts[users[index].ID] = r.users.ClientsCount(users[index].ID)
	}

	return responses.Paginated(ctx, resources.Users(users, counts), pagination, total)
}

// Store invites a user: the row is created pending and an activation code is
// mailed out. No password is ever chosen here.
func (r *UserController) Store(ctx http.Context) http.Response {
	var request requests.CreateUserRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	email := services.NormalizeEmail(request.Email)
	taken, err := r.users.EmailTaken(email, 0)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}
	if taken {
		return responses.InvalidField(ctx, "email", "email is already taken")
	}

	role, err := r.users.RoleBySlug(request.Role)
	if err != nil {
		return responses.InvalidField(ctx, "role", err.Error())
	}

	user := &models.User{
		Name:   request.Name,
		Email:  email,
		Status: models.UserStatusPending,
		RoleID: &role.ID,
	}

	// The account and its activation code are created together. An invite that
	// could not be mailed leaves a pending account nobody can activate and
	// whose address is then taken, so the whole thing rolls back instead.
	err = facades.Orm().Transaction(func(tx orm.Query) error {
		if err := tx.Create(user); err != nil {
			return err
		}

		return r.otp.IssueWithin(tx, services.OtpIssueInput{
			Email:   user.Email,
			Purpose: models.OtpPurposeVerify,
			Name:    user.Name,
			UserID:  &user.ID,
		})
	})
	if err != nil {
		if errors.Is(err, services.ErrOtpResendTooSoon) {
			return responses.Error(ctx, http.StatusTooManyRequests, err.Error())
		}

		return MailFailure(ctx, err)
	}
	user.Role = role

	id := user.ID
	r.audit.Log(Actor(ctx), "user.create", "user", &id,
		map[string]any{"email": email, "role": role.Slug}, ctx.Request().Ip())
	r.audit.Log(Actor(ctx), "otp.send", "user", &id,
		map[string]any{"purpose": models.OtpPurposeVerify}, ctx.Request().Ip())

	return responses.Data(ctx, http.StatusCreated, resources.User(user, 0))
}

// Show returns one account.
func (r *UserController) Show(ctx http.Context) http.Response {
	user, found := r.find(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	r.users.LoadRole(user)

	return responses.Data(ctx, http.StatusOK, resources.User(user, r.users.ClientsCount(user.ID)))
}

// Update changes the name, role or status of an account.
func (r *UserController) Update(ctx http.Context) http.Response {
	user, found := r.find(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	var request requests.UpdateUserRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	self := AuthUserID(ctx) != nil && *AuthUserID(ctx) == user.ID
	details := map[string]any{}

	if request.Name != nil && *request.Name != "" {
		user.Name = *request.Name
		details["name"] = *request.Name
	}

	// Everything that could take the last super admin out of the panel is
	// checked before anything is written, so a refusal leaves the row alone.
	wasSuperAdmin := r.users.IsActiveSuperAdmin(user)
	losesSuperAdmin := wasSuperAdmin && request.Role != nil && *request.Role != "" &&
		*request.Role != services.SuperAdminSlug
	losesActive := wasSuperAdmin && request.Status != nil && *request.Status != "" &&
		*request.Status != models.UserStatusActive
	if losesSuperAdmin || losesActive {
		others, err := r.users.ActiveSuperAdminsExcept(user.ID)
		if err != nil {
			return responses.Error(ctx, http.StatusInternalServerError, err.Error())
		}
		if others == 0 {
			return responses.Error(ctx, http.StatusUnprocessableEntity, lastSuperAdminMessage)
		}
	}

	if request.Role != nil && *request.Role != "" {
		// Demoting yourself would take the last super admin out of the panel
		// in one request, so it is refused outright.
		if self {
			return responses.InvalidField(ctx, "role", "you cannot change your own role")
		}

		role, err := r.users.RoleBySlug(*request.Role)
		if err != nil {
			return responses.InvalidField(ctx, "role", err.Error())
		}
		user.RoleID = &role.ID
		user.Role = role
		details["role"] = role.Slug
	}

	if request.Status != nil && *request.Status != "" {
		if self {
			return responses.InvalidField(ctx, "status", "you cannot change your own status")
		}
		// A pending invite is not activated by hand; it is activated by the
		// code the invited user receives.
		if user.Status == models.UserStatusPending && *request.Status == models.UserStatusActive {
			return responses.InvalidField(ctx, "status", "the user has to verify its email first")
		}
		user.Status = *request.Status
		details["status"] = *request.Status

		// A blocked account must not keep working until its token expires.
		if *request.Status != models.UserStatusActive {
			r.users.TouchCredentials(user)
		}
	}

	if err := facades.Orm().Query().Save(user); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	r.users.LoadRole(user)

	id := user.ID
	action := "user.update"
	if request.Status != nil && *request.Status == models.UserStatusBlocked {
		action = "user.block"
	}
	r.audit.Log(Actor(ctx), action, "user", &id, details, ctx.Request().Ip())

	return responses.Data(ctx, http.StatusOK, resources.User(user, r.users.ClientsCount(user.ID)))
}

// ResendOtp mails a fresh activation code to a pending account.
func (r *UserController) ResendOtp(ctx http.Context) http.Response {
	user, found := r.find(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	if user.Status != models.UserStatusPending {
		return responses.InvalidField(ctx, "status", "the user is not pending")
	}

	if err := r.sendInvite(ctx, user); err != nil {
		return err
	}

	return responses.NoContent(ctx)
}

// Destroy removes an account. Clients it owns have to be handed to somebody
// else in the same transaction, otherwise they would end up unowned and
// invisible to every admin.
func (r *UserController) Destroy(ctx http.Context) http.Response {
	user, found := r.find(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	if AuthUserID(ctx) != nil && *AuthUserID(ctx) == user.ID {
		return responses.Error(ctx, http.StatusUnprocessableEntity, "you cannot delete yourself")
	}

	if r.users.IsActiveSuperAdmin(user) {
		others, err := r.users.ActiveSuperAdminsExcept(user.ID)
		if err != nil {
			return responses.Error(ctx, http.StatusInternalServerError, err.Error())
		}
		if others == 0 {
			return responses.Error(ctx, http.StatusUnprocessableEntity, lastSuperAdminMessage)
		}
	}

	owned := r.users.ClientsCount(user.ID)
	var target *models.User
	if owned > 0 {
		reassignTo := uint(ctx.Request().QueryInt("reassign_to", 0))
		if reassignTo == 0 {
			return responses.InvalidField(ctx, "reassign_to",
				"the user owns clients, pass reassign_to with the id of their new owner")
		}
		if reassignTo == user.ID {
			return responses.InvalidField(ctx, "reassign_to", "reassign_to must be another user")
		}

		var candidate models.User
		if err := facades.Orm().Query().Where("id", reassignTo).First(&candidate); err != nil || candidate.ID == 0 {
			return responses.InvalidField(ctx, "reassign_to", "user not found")
		}
		if candidate.Status != models.UserStatusActive {
			return responses.InvalidField(ctx, "reassign_to", "the new owner has to be an active user")
		}
		target = &candidate
	}

	deleted := user.ID
	err := facades.Orm().Transaction(func(tx orm.Query) error {
		if target != nil {
			if _, err := tx.Model(&models.Client{}).
				Where("owner_id", deleted).Update("owner_id", target.ID); err != nil {
				return err
			}
		}

		_, err := tx.Delete(user)

		return err
	})
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	details := map[string]any{"email": user.Email}
	if target != nil {
		details["reassigned_to"] = target.ID
		details["clients"] = owned
	}
	r.audit.Log(Actor(ctx), "user.delete", "user", &deleted, details, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

// sendInvite issues an activation code and maps the resend gap onto 429.
func (r *UserController) sendInvite(ctx http.Context, user *models.User) http.Response {
	err := r.otp.Issue(services.OtpIssueInput{
		Email:   user.Email,
		Purpose: models.OtpPurposeVerify,
		Name:    user.Name,
		UserID:  &user.ID,
	})
	if err != nil {
		if errors.Is(err, services.ErrOtpResendTooSoon) {
			return responses.Error(ctx, http.StatusTooManyRequests, err.Error())
		}

		return MailFailure(ctx, err)
	}

	id := user.ID
	r.audit.Log(Actor(ctx), "otp.send", "user", &id,
		map[string]any{"purpose": models.OtpPurposeVerify}, ctx.Request().Ip())

	return nil
}

func (r *UserController) find(ctx http.Context) (*models.User, bool) {
	id := ctx.Request().RouteInt("id")
	if id <= 0 {
		return nil, false
	}

	var user models.User
	if err := facades.Orm().Query().Where("id", id).First(&user); err != nil || user.ID == 0 {
		return nil, false
	}

	return &user, true
}
