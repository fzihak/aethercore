package core

import (
	"context"
	"errors"
	"testing"

	"github.com/fzihak/aethercore/core/llm"
)

type MockVerifier struct {
	fail         bool
	wantSig      string
	receivedSig  string
	receivedJSON []byte
}

func (m *MockVerifier) Verify(manifestJSON []byte, signatureHex string) (bool, error) {
	m.receivedSig = signatureHex
	m.receivedJSON = manifestJSON
	if m.fail {
		return false, errors.New("signature mismatch")
	}
	if m.wantSig != "" && signatureHex != m.wantSig {
		return false, errors.New("wrong signature")
	}
	return true, nil
}

type DummyTool struct{}

func (d *DummyTool) Manifest() llm.ToolManifest                               { return llm.ToolManifest{Name: "dummy"} }
func (d *DummyTool) Execute(ctx context.Context, args string) (string, error) { return "", nil }

func TestToolRegistry_BlocksUnverified(t *testing.T) {
	registry := NewToolRegistry(&MockVerifier{fail: true})
	err := registry.Register(NewVerifiedTool(&DummyTool{}, "deadbeef"))
	if err == nil {
		t.Errorf("Expected unverified tool to be blocked")
	}
}

func TestToolRegistry_BlocksUnsignedToolWhenVerifierEnabled(t *testing.T) {
	registry := NewToolRegistry(&MockVerifier{})
	err := registry.Register(&DummyTool{})
	if err == nil {
		t.Fatalf("expected unsigned tool to be rejected")
	}
}

func TestToolRegistry_PassesSignatureToVerifier(t *testing.T) {
	verifier := &MockVerifier{wantSig: "abcd1234"}
	registry := NewToolRegistry(verifier)
	err := registry.Register(NewVerifiedTool(&DummyTool{}, "abcd1234"))
	if err != nil {
		t.Fatalf("expected signed tool to be accepted, got %v", err)
	}
	if verifier.receivedSig != "abcd1234" {
		t.Fatalf("expected verifier to receive signature, got %q", verifier.receivedSig)
	}
	if len(verifier.receivedJSON) == 0 {
		t.Fatalf("expected manifest JSON to be provided to verifier")
	}
	tool, err := registry.Get("dummy")
	if err != nil {
		t.Fatalf("expected registered tool to be retrievable: %v", err)
	}
	if tool == nil {
		t.Fatalf("expected non-nil tool from registry")
	}
}
