package security

import (
	"context"
	"testing"
)

func TestRegexScanner_SafeInput(t *testing.T) {
	scanner := NewRegexScanner()
	result := scanner.Scan(context.Background(), "Hello, can you help me write a python script?", GuardConfig{})
	if !result.IsSafe {
		t.Errorf("Expected safe input to pass, got IsSafe=false")
	}
}
