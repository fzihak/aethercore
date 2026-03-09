package audit

import "time"

// AuditEvent represents a single immutable action within the system.
type AuditEvent struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	Actor     string                 `json:"actor"`
	Metadata  map[string]interface{} `json:"metadata"`
}
