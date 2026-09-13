package models

import (
	"github.com/goravel/framework/support/carbon"
)

type AuditLog struct {
	ID uint `json:"id" gorm:"primaryKey"`
	// ActorType is "user", "client" or "system".
	ActorType   string  `json:"actor_type"`
	ActorID     *uint   `json:"actor_id"`
	ActorName   *string `json:"actor_name"`
	Action      string  `json:"action"`
	SubjectType *string `json:"subject_type"`
	SubjectID   *uint   `json:"subject_id"`
	// ClientID is the client an entry belongs to, when there is one. It is what
	// the audit listing is scoped by for the admin role.
	ClientID  *uint            `json:"client_id"`
	Details   map[string]any   `json:"details" gorm:"serializer:json"`
	IP        *string          `json:"ip" gorm:"column:ip"`
	CreatedAt *carbon.DateTime `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

func (r *AuditLog) TableName() string {
	return "audit_logs"
}
