package core

import (
	"context"
	"testing"

	"github.com/fzihak/aethercore/core/llm"
)

type MockVerifier struct {
	fail bool
}

func (m *MockVerifier) Verify(manifestJSON []byte, signatureHex string) (bool, error) {
	return !m.fail, nil
}

type DummyTool struct{}

func (d *DummyTool) Manifest() llm.ToolManifest                               { return llm.ToolManifest{Name: "dummy"} }
func (d *DummyTool) Execute(ctx context.Context, args string) (string, error) { return "", nil }

func TestToolRegistry_BlocksUnverified(t *testing.T) {
	registry := NewToolRegistry(&MockVerifier{fail: true})
	err := registry.Register(&DummyTool{})
	if err == nil {
		t.Errorf("Expected unverified tool to be blocked")
	}
}
