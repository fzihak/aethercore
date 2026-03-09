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
	specialChars := 0
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\t' {
			if currentWordLen > maxWordLen { maxWordLen = currentWordLen }
			currentWordLen = 0
		} else {
			currentWordLen++
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
				specialChars++
			}
		}
	}
	if currentWordLen > maxWordLen { maxWordLen = currentWordLen }

	if maxWordLen > 50 {
		return GuardResult{
			IsSafe: false, Confidence: 0.8,
			Violations: []AdversarialMatch{{Category: "TOKEN_DENSITY_ANOMALY", Description: "Unusually long unbroken token detected", Severity: "MEDIUM"}},
		}
	}
	
	ratio := float64(specialChars) / float64(len(text))
	if len(text) > 20 && ratio > 0.4 {
		return GuardResult{
			IsSafe: false, Confidence: 0.85,
			Violations: []AdversarialMatch{{Category: "PADDING_ABUSE", Description: "High concentration of special characters", Severity: "HIGH"}},
		}
	}
	return GuardResult{IsSafe: true}
}
