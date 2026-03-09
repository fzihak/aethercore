package security

import (
	"context"
	"regexp"
)

type RegexScanner struct {
	patterns map[string]*regexp.Regexp
}

func NewRegexScanner() *RegexScanner {
	return &RegexScanner{
		patterns: make(map[string]*regexp.Regexp),
	}
}

func (s *RegexScanner) Scan(ctx context.Context, text string, config GuardConfig) GuardResult {
	return GuardResult{IsSafe: true}
}
