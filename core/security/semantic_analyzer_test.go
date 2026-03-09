package security

import (
	"context"
	"testing"
)

func TestSemanticAnalyzer_SafeInput(t *testing.T) {
	analyzer := NewSemanticAnalyzer()
	result := analyzer.Scan(context.Background(), "This is a completely normal sentence.", GuardConfig{})
	if !result.IsSafe {
		t.Errorf("Expected IsSafe=true")
	}
}
