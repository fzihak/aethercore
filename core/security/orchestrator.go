package security

import "context"

type OrchestratorGuard struct {
	scanners []PromptGuard
}

func NewOrchestratorGuard(scanners ...PromptGuard) *OrchestratorGuard {
	return &OrchestratorGuard{scanners: scanners}
}

func (o *OrchestratorGuard) Scan(ctx context.Context, text string, config GuardConfig) GuardResult {
	return GuardResult{IsSafe: true}
}
