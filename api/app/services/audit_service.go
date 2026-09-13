package services

import (
	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

// Actor identifies who triggered an audited action.
type Actor struct {
	Type string
	ID   *uint
	Name *string
}

// UserActor builds an actor for an authenticated admin user.
func UserActor(user *models.User) Actor {
	if user == nil {
		return Actor{Type: models.ActorTypeSystem}
	}

	id := user.ID
	name := user.Name

	return Actor{Type: models.ActorTypeUser, ID: &id, Name: &name}
}

// ClientActor builds an actor for an API client.
func ClientActor(client *models.Client) Actor {
	if client == nil {
		return Actor{Type: models.ActorTypeSystem}
	}

	id := client.ID
	actor := Actor{Type: models.ActorTypeClient, ID: &id}
	if client.Name != nil {
		actor.Name = client.Name
	} else {
		actor.Name = client.Username
	}

	return actor
}

// SystemActor is used by console commands and background jobs.
func SystemActor() Actor {
	return Actor{Type: models.ActorTypeSystem}
}

type AuditService struct{}

func NewAuditService() *AuditService {
	return &AuditService{}
}

// Log records one audited action. Failures are logged but never bubble up, an
// audit trail problem must not break the request that caused it.
//
// The client an entry belongs to is derived when it can be: an entry about a
// client, or one a client itself produced. Everything else has to say so with
// LogFor, otherwise the entry stays invisible to the admin role.
func (r *AuditService) Log(actor Actor, action, subjectType string, subjectID *uint, details map[string]any, ip string) {
	var clientID *uint
	if subjectType == "client" {
		clientID = subjectID
	} else if actor.Type == models.ActorTypeClient {
		clientID = actor.ID
	}

	r.LogFor(actor, action, subjectType, subjectID, clientID, details, ip)
}

// LogFor records an audited action against a known client, which is what the
// ownership scoping of the audit listing filters by.
func (r *AuditService) LogFor(actor Actor, action, subjectType string, subjectID, clientID *uint, details map[string]any, ip string) {
	if details == nil {
		details = map[string]any{}
	}

	entry := &models.AuditLog{
		ClientID:  clientID,
		ActorType: actor.Type,
		ActorID:   actor.ID,
		ActorName: actor.Name,
		Action:    action,
		SubjectID: subjectID,
		Details:   details,
	}

	if subjectType != "" {
		entry.SubjectType = &subjectType
	}
	if ip != "" {
		entry.IP = &ip
	}

	if err := facades.Orm().Query().Create(entry); err != nil {
		facades.Log().With(map[string]any{"action": action}).Error("failed to write audit log: " + err.Error())
	}
}
