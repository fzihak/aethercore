package security

import "context"

type OrchestratorGuard struct {
	scanners []PromptGuard
}

func NewOrchestratorGuard(scanners ...PromptGuard) *OrchestratorGuard {
	return &OrchestratorGuard{scanners: scanners}
}

func (o *OrchestratorGuard) Scan(ctx context.Context, text string, config GuardConfig) GuardResult {
	for _, scanner := range o.scanners {
		res := scanner.Scan(ctx, text, config)
		if !res.IsSafe {
			return res
		}
	}
	return GuardResult{IsSafe: true}
}
