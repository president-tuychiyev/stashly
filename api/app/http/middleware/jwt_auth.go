package middleware

import (
	"strings"

	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

// TokenIssuedAtKey is the context key jwt_auth stores the token's "iat" claim
// under.
const TokenIssuedAtKey = "token_issued_at"

type jwtAuth struct{}

// JwtAuth parses the bearer token of the request so that the guard, and every
// controller after it, can resolve the authenticated user.
func JwtAuth() http.Middleware {
	return &jwtAuth{}
}

func (r *jwtAuth) Signature() string {
	return "jwt_auth"
}

func (r *jwtAuth) Handle(ctx http.Context) {
	token := strings.TrimSpace(ctx.Request().Header("Authorization"))
	if token == "" {
		ctx.Request().AbortWithStatusJson(http.StatusUnauthorized, http.Json{"message": "Unauthenticated"})
		return
	}

	payload, err := facades.Auth(ctx).Parse(token)
	if err != nil || payload == nil {
		ctx.Request().AbortWithStatusJson(http.StatusUnauthorized, http.Json{"message": "Unauthenticated"})
		return
	}

	// can_user compares this against the user's credentials_changed_at, which
	// is how a password reset, an email change or a block ends the sessions
	// that were already open. The payload is only available here, where the
	// token is parsed.
	ctx.WithValue(TokenIssuedAtKey, payload.IssuedAt)

	ctx.Request().Next()
}
