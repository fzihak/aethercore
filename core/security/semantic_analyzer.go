package security

import "context"

type SemanticAnalyzer struct {}

func NewSemanticAnalyzer() *SemanticAnalyzer { return &SemanticAnalyzer{} }
func (s *SemanticAnalyzer) Scan(ctx context.Context, text string, config GuardConfig) GuardResult {
	return GuardResult{IsSafe: true}
}
