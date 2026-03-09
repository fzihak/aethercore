package security

import "context"

type PromptGuard interface {
	Scan(ctx context.Context, text string, config GuardConfig) GuardResult
}

type GuardConfig struct {
	StrictnessLevel   int
	BypassTokens      []string
	MaxHeuristicScore float64
}

type GuardResult struct {
	IsSafe     bool
	Confidence float64
	Violations []AdversarialMatch
}

type AdversarialMatch struct {
	Category    string
	Description string
	Snippet     string
	Severity    string
}
