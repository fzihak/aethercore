package security

import "context"

type SemanticAnalyzer struct {}

func NewSemanticAnalyzer() *SemanticAnalyzer { return &SemanticAnalyzer{} }
func (s *SemanticAnalyzer) Scan(ctx context.Context, text string, config GuardConfig) GuardResult {
	if len(text) == 0 {
		return GuardResult{IsSafe: true}
	}
	maxWordLen := 0
	currentWordLen := 0
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\t' {
			if currentWordLen > maxWordLen { maxWordLen = currentWordLen }
			currentWordLen = 0
		} else {
			currentWordLen++
		}
	}
	if currentWordLen > maxWordLen { maxWordLen = currentWordLen }

	if maxWordLen > 50 {
		return GuardResult{
			IsSafe: false, Confidence: 0.8,
			Violations: []AdversarialMatch{{Category: "TOKEN_DENSITY_ANOMALY", Description: "Unusually long unbroken token detected", Severity: "MEDIUM"}},
		}
	}
	return GuardResult{IsSafe: true}
}
