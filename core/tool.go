package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// Inviolable Rule: Layer 0 strictly uses Go stdlib ONLY.

var (
	ErrNilTool        = errors.New("cannot register nil tool")
	ErrToolRegistered = errors.New("tool already registered")
	ErrToolNotFound   = errors.New("tool not found")
)

// Capability flags define what a tool is permitted to do.
type Capability string

const (
	CapNetwork    Capability = "network"
	CapFilesystem Capability = "filesystem"
	CapState      Capability = "state"
)

// ToolManifest defines the declarative capability-envelope for a tool.
// This is strictly enforced by the Kernel before the Tool executes.
type ToolManifest struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Parameters   json.RawMessage `json:"parameters"` // JSON Schema for the tool parameters
	Capabilities []Capability    `json:"capabilities"`
	MaxRuntimeMs int             `json:"max_runtime_ms"`
	MemoryLimit  int             `json:"memory_limit_mb"`
}

// Tool is the interface all in-process and sandboxed tools must implement.
type Tool interface {
	Manifest() ToolManifest
	Execute(ctx context.Context, args string) (string, error)
}

// ToolRegistry manages the available tools in an ephemeral execution.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

func (r *ToolRegistry) Register(t Tool) error {
	if t == nil {
		return ErrNilTool
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	m := t.Manifest()
	if _, exists := r.tools[m.Name]; exists {
		return fmt.Errorf("%w: %s", ErrToolRegistered, m.Name)
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

func (r *ToolRegistry) Manifests() []ToolManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	manifests := make([]ToolManifest, 0, len(r.tools))
	for _, t := range r.tools {
		manifests = append(manifests, t.Manifest())
	}
	return manifests
}
