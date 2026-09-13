package middleware

import (
	"errors"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"
	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// superAdminSlug and adminSlug are the two roles the seeder ships. Any other
// role falls back to the explicit permission list on the role itself.
const (
	superAdminSlug = services.SuperAdminSlug
	adminSlug      = "admin"
)

// usersRoutePrefix names the endpoints only a super admin may reach.
const usersRoutePrefix = "admin.users."

type canUser struct{}

func CanUser() http.Middleware {
	return &canUser{}
}

func (r *canUser) Signature() string {
	return "can_user"
}

func (r *canUser) Handle(ctx http.Context) {
	var user models.User
	if err := facades.Auth(ctx).User(&user); err != nil || user.ID == 0 {
		ctx.Request().AbortWithStatusJson(http.StatusUnauthorized, http.Json{"message": "Unauthenticated"})
		return
	}

	// A token stays valid until it expires, so a user blocked after signing in
	// would keep working without this check.
	if user.Status != models.UserStatusActive {
		forbid(ctx)
		return
	}

	// goravel's JWT payload is fixed (key and subject), so a token version
	// cannot be carried inside the token. The token's issue time is compared
	// against the account's last credential change instead: anything older was
	// handed out before the password, the address or the block changed and is
	// refused.
	if TokenPredatesCredentials(ctx, &user) {
		ctx.Request().AbortWithStatusJson(http.StatusUnauthorized, http.Json{"message": "Unauthenticated"})
		return
	}

	routeName := ctx.Request().Name()
	if routeName == "" {
		forbid(ctx)
		return
	}

	if user.RoleID == nil {
		forbid(ctx)
		return
	}

	role, err := resolveRole(*user.RoleID)
	if err != nil || !role.IsActive {
		forbid(ctx)
		return
	}

	if role.Slug == superAdminSlug {
		next(ctx, &user, &role)
		return
	}

	// The admin role reaches every admin endpoint except user management;
	// what it may see inside those endpoints is decided by the ownership
	// scoping in the controllers, not here.
	if role.Slug == adminSlug {
		if strings.HasPrefix(routeName, usersRoutePrefix) {
			forbid(ctx)
			return
		}

		next(ctx, &user, &role)
		return
	}

	for _, permission := range role.Permissions {
		if permission == routeName {
			next(ctx, &user, &role)
			return
		}
	}

	forbid(ctx)
}

// TokenPredatesCredentials reports whether the token of the request was issued
// before the account's credentials last changed.
//
// The "iat" claim only has second resolution, so credentials_changed_at is
// truncated to the second as well: without that, changing a password and
// signing back in inside the same second would reject the brand new token.
func TokenPredatesCredentials(ctx http.Context, user *models.User) bool {
	if user == nil || user.CredentialsChangedAt == nil {
		return false
	}

	issuedAt, ok := ctx.Value(TokenIssuedAtKey).(time.Time)
	if !ok || issuedAt.IsZero() {
		return false
	}

	return issuedAt.Before(user.CredentialsChangedAt.StdTime().Truncate(time.Second))
}

func resolveRole(roleID uint) (models.Role, error) {
	role := models.Role{}
	cacheKey := services.RoleCacheKey(roleID)

	// Cache().Get returns any, never an error, so the cached value has to be
	// type asserted back into a role before it can be trusted.
	if cached, ok := facades.Cache().Get(cacheKey).(models.Role); ok && cached.ID != 0 {
		return cached, nil
	}

	if err := facades.Orm().Query().Where("id", roleID).First(&role); err != nil {
		return role, err
	}
	if role.ID == 0 {
		return role, errors.New("role not found")
	}

	facades.Cache().Put(cacheKey, role, 10*time.Minute)

	return role, nil
}

func next(ctx http.Context, user *models.User, role *models.Role) {
	user.Role = role
	ctx.WithValue("auth_user", user)
	ctx.WithValue("auth_user_id", user.ID)
	ctx.WithValue("auth_role", role.Slug)
	ctx.Request().Next()
}

func forbid(ctx http.Context) {
	ctx.Request().AbortWithStatusJson(http.StatusForbidden, http.Json{"message": "Forbidden"})
}
