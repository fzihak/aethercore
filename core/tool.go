package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/fzihak/aethercore/core/llm"
	"github.com/fzihak/aethercore/core/security"
)

// Inviolable Rule: Layer 0 strictly uses Go stdlib ONLY.

var (
	ErrNilTool        = errors.New("cannot register nil tool")
	ErrToolRegistered = errors.New("tool already registered")
	ErrToolNotFound   = errors.New("tool not found")
)

// Tool is the interface all in-process and sandboxed tools must implement.
type Tool interface {
	Manifest() llm.ToolManifest
	Execute(ctx context.Context, args string) (string, error)
}

// SignedTool carries the detached Ed25519 signature for a tool manifest.
// Registries with an attached verifier require this interface to be present.
type SignedTool interface {
	Tool
	Signature() string
}

// ToolRegistry manages the available tools in an ephemeral execution.
type ToolRegistry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	verifier security.ToolVerifier
}

func NewToolRegistry(verifier security.ToolVerifier) *ToolRegistry {
	return &ToolRegistry{
		tools:    make(map[string]Tool),
		verifier: verifier,
	}
}

func (r *ToolRegistry) SetVerifier(verifier security.ToolVerifier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verifier = verifier
}

func (r *ToolRegistry) Register(t Tool) error {
	if t == nil {
		return ErrNilTool
	}

	m := t.Manifest()
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[m.Name]; exists {
		return fmt.Errorf("%w: %s", ErrToolRegistered, m.Name)
	}

	if r.verifier != nil {
		signedTool, ok := t.(SignedTool)
		if !ok {
			slog.Warn("tool_verification_failed", slog.String("tool", m.Name), slog.String("error", "missing detached signature"))
			return fmt.Errorf("cryptographic verification failed for tool %s: missing detached signature", m.Name)
		}

		manifestBytes, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("tool_marshal_failed: %w", err)
		}
		verified, err := r.verifier.Verify(manifestBytes, signedTool.Signature())
		if !verified || err != nil {
			errStr := "signature verification rejected"
			if err != nil {
				errStr = err.Error()
			}
			slog.Warn("tool_verification_failed", slog.String("tool", m.Name), slog.String("error", errStr))
			if err != nil {
				return fmt.Errorf("cryptographic verification failed for tool %s: %w", m.Name, err)
			}
			return fmt.Errorf("cryptographic verification failed for tool %s: %s", m.Name, errStr)
		}
	}
	r.tools[m.Name] = t
	return nil
}

func (r *ToolRegistry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}
	return t, nil
}

func (r *ToolRegistry) Manifests() []llm.ToolManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	manifests := make([]llm.ToolManifest, 0, len(r.tools))
	for _, t := range r.tools {
		manifests = append(manifests, t.Manifest())
	}
	return manifests
}
