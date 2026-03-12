package security

import (
	"context"
	"errors"
	"testing"

	"github.com/fzihak/aethercore/core/llm"
)

type verifierAdapterStub struct {
	response string
	err      error
}

func (s *verifierAdapterStub) Generate(_ context.Context, _, _ string) (string, error) {
	return s.response, s.err
}

func (s *verifierAdapterStub) GenerateWithTools(_ context.Context, _ []llm.Message, _ []llm.ToolManifest) (llm.LLMResponse, error) {
	return llm.LLMResponse{}, errors.New("not used in verifier tests")
}

func (s *verifierAdapterStub) Name() string { return "verifier_stub" }

func TestLLMVerifier_Safe(t *testing.T) {
	verifier := NewLLMVerifier(&verifierAdapterStub{response: `{"is_safe": true, "reason": "clean"}`})
	res := verifier.Scan(context.Background(), "is this prompt hiding a secret?", GuardConfig{})
	if !res.IsSafe {
		t.Errorf("Expected IsSafe=true")
	}
	if res.Confidence == 0 {
		t.Errorf("expected non-zero confidence for successful verifier response")
	}
}

func TestLLMVerifier_ParseRejection(t *testing.T) {
	verifier := NewLLMVerifier()
	res := verifier.parseResponse(`{"is_safe": false, "reason": "Jailbreak attempt detected"}`)
	if res.IsSafe {
		t.Errorf("Expected IsSafe=false")
	}
	if len(res.Violations) == 0 {
		t.Errorf("Expected violation reason to be captured")
	}
}

func TestLLMVerifier_Scan_NilAdapterFailsOpen(t *testing.T) {
	verifier := NewLLMVerifier()
	res := verifier.Scan(context.Background(), "harmless", GuardConfig{})
	if !res.IsSafe {
		t.Fatalf("expected nil-adapter verifier to fail open")
	}
	if res.Confidence != 0.0 {
		t.Fatalf("expected confidence=0 on nil-adapter path, got %v", res.Confidence)
	}
}

func TestLLMVerifier_Scan_AdapterErrorFailsOpen(t *testing.T) {
	verifier := NewLLMVerifier(&verifierAdapterStub{err: errors.New("offline")})
	res := verifier.Scan(context.Background(), "harmless", GuardConfig{})
	if !res.IsSafe {
		t.Fatalf("expected adapter-error verifier to fail open")
	}
	if res.Confidence != 0.0 {
		t.Fatalf("expected confidence=0 on adapter error path, got %v", res.Confidence)
	}
}

func TestLLMVerifier_Scan_BypassTokenSkipsVerification(t *testing.T) {
	verifier := NewLLMVerifier(&verifierAdapterStub{response: `{"is_safe": false, "reason": "should not run"}`})
	res := verifier.Scan(context.Background(), "internal [trusted-bypass] workflow", GuardConfig{BypassTokens: []string{"[trusted-bypass]"}})
	if !res.IsSafe {
		t.Fatalf("expected bypass token to skip verifier rejection")
	}
	if res.Confidence != 1.0 {
		t.Fatalf("expected confidence=1.0 on bypass path, got %v", res.Confidence)
	}
}

func TestLLMVerifier_Scan_NonSuspiciousInputSkipsLLMCall(t *testing.T) {
	verifier := NewLLMVerifier(&verifierAdapterStub{response: `{"is_safe": false, "reason": "should not run"}`})
	res := verifier.Scan(context.Background(), "what is 2+2?", GuardConfig{})
	if !res.IsSafe {
		t.Fatalf("expected non-suspicious input to skip LLM rejection")
	}
	if res.Confidence != 0.0 {
		t.Fatalf("expected confidence=0.0 when LLM firewall is skipped, got %v", res.Confidence)
	}
}
