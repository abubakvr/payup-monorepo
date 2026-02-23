package kafka

import "time"

// AuditEvent matches the payload expected by the audit service (topic: audit-events).
type AuditEvent struct {
	Service       string                 `json:"service"`
	UserID        *string                `json:"user_id,omitempty"`
	Action        string                 `json:"action"`
	Entity        string                 `json:"entity"`
	EntityID      *string                `json:"entity_id,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CorrelationID *string                `json:"correlation_id,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
}
