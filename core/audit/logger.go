package audit

import "context"

// AuditLogger defines the interface for the cryptographically linked append-only audit trail.
type AuditLogger interface {
	LogEvent(ctx context.Context, event AuditEvent) error
	VerifyChain() (bool, error)
}
