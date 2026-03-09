package security

import (
	"context"
	"testing"
)

func TestLLMVerifier_Safe(t *testing.T) {
	verifier := NewLLMVerifier()
	res := verifier.Scan(context.Background(), "test", GuardConfig{})
	if !res.IsSafe {
		t.Errorf("Expected IsSafe=true")
	}
}
