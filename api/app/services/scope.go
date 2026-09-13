package services

import (
	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

// SuperAdminSlug is the role that is exempt from ownership scoping.
const SuperAdminSlug = "super_admin"

// Ownership describes what the caller of an /admin route may see. Every admin
// controller asks for it and filters by it, so the rule lives in one place
// instead of being re-derived per endpoint.
type Ownership struct {
	// UserID is the authenticated admin user.
	UserID uint
	// IsSuperAdmin lifts every ownership filter.
	IsSuperAdmin bool
}

// Scope reads the ownership of the current request out of the context that
// can_user filled in.
func Scope(ctx http.Context) Ownership {
	out := Ownership{}

	if id, ok := ctx.Value("auth_user_id").(uint); ok {
		out.UserID = id
	}
	if slug, ok := ctx.Value("auth_role").(string); ok && slug == SuperAdminSlug {
		out.IsSuperAdmin = true
	}

	return out
}

// OwnsClient reports whether the caller may touch a client.
func (r Ownership) OwnsClient(client *models.Client) bool {
	if r.IsSuperAdmin {
		return true
	}
	if client == nil || client.OwnerID == nil {
		return false
	}

	return *client.OwnerID == r.UserID
}

// ApplyToClients narrows a query over the clients table.
func (r Ownership) ApplyToClients(query orm.Query) orm.Query {
	if r.IsSuperAdmin {
		return query
	}

	return query.Where("owner_id", r.UserID)
}

// ApplyToClientColumn narrows a query over any table that carries a client_id
// column, e.g. files or archives.
func (r Ownership) ApplyToClientColumn(query orm.Query, column string) orm.Query {
	if r.IsSuperAdmin {
		return query
	}

	return query.WhereIn(column, r.clientIDs())
}

// ApplyToAuditLogs narrows the audit trail: an admin sees what it did itself
// plus everything recorded against one of its own clients.
func (r Ownership) ApplyToAuditLogs(query orm.Query) orm.Query {
	if r.IsSuperAdmin {
		return query
	}

	ids := r.clientIDs()
	userID := r.UserID

	return query.Where(func(inner orm.Query) orm.Query {
		return inner.
			Where(func(mine orm.Query) orm.Query {
				return mine.Where("actor_type", models.ActorTypeUser).Where("actor_id", userID)
			}).
			OrWhereIn("client_id", ids)
	})
}

// MayUseClient reports whether the caller may act on a client id, loading the
// client to check its owner.
func (r Ownership) MayUseClient(clientID uint) bool {
	if r.IsSuperAdmin {
		return true
	}

	exists, err := facades.Orm().Query().Model(&models.Client{}).
		Where("id", clientID).Where("owner_id", r.UserID).Exists()

	return err == nil && exists
}

// clientIDs lists the clients the caller owns. The empty list is returned as
// a single impossible id so that an IN clause built from it matches nothing
// instead of being dropped by the query builder.
func (r Ownership) clientIDs() []any {
	var clients []models.Client
	if err := facades.Orm().Query().Model(&models.Client{}).
		Where("owner_id", r.UserID).Select("id").Get(&clients); err != nil {
		facades.Log().Warning("failed to resolve owned clients: " + err.Error())
	}

	out := make([]any, 0, len(clients))
	for index := range clients {
		out = append(out, clients[index].ID)
	}
	if len(out) == 0 {
		out = append(out, uint(0))
	}

	return out
}
