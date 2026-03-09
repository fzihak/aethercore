package security

import (
	"context"
	"regexp"
	"sync"
)

type RegexScanner struct {}

var (
	compiledPatterns map[string]*regexp.Regexp
	once             sync.Once
)

func NewRegexScanner() *RegexScanner {
	once.Do(func() {
		compiledPatterns = map[string]*regexp.Regexp{
			"SYSTEM_PROMPT_LEAK":  regexp.MustCompile(`(?i)(reveal|show|print|output)\s+(your\s+)?(system\s+)?(prompt|instructions)`),
			"IGNORE_INSTRUCTIONS": regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous\s+)?(instructions|directions|rules)`),
			"ROLEPLAY_JAILBREAK":  regexp.MustCompile(`(?i)(you\s+are\s+now|act\s+as\s+a)\s+(dan|do\s+anything\s+now|developer\s+mode|unrestricted)`),
		}
	})
	return &RegexScanner{}
}

func (s *RegexScanner) Scan(ctx context.Context, text string, config GuardConfig) GuardResult {
	for category, pattern := range compiledPatterns {
		if loc := pattern.FindStringIndex(text); loc != nil {
			snippet := text[loc[0]:loc[1]]
			return GuardResult{
				IsSafe: false, Confidence: 0.9,
				Violations: []AdversarialMatch{{Category: category, Description: "Matched regex", Snippet: snippet, Severity: "HIGH"}},
			}
		}
	}
	return GuardResult{IsSafe: true}
}
