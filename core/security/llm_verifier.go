package security

import "context"

type LLMVerifier struct {}

func NewLLMVerifier() *LLMVerifier { return &LLMVerifier{} }

func (s *LLMVerifier) Scan(ctx context.Context, text string, config GuardConfig) GuardResult {
	return GuardResult{IsSafe: true}
}
