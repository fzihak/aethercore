package core

import (
	"context"
	"encoding/json"
	"errors"
)

// Inviolable Rule: Layer 0 strictly uses Go stdlib ONLY.

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
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

func (r *ToolRegistry) Register(t Tool) error {
	m := t.Manifest()
	if _, exists := r.tools[m.Name]; exists {
		return errors.New("tool already registered: " + m.Name)
	}
	r.tools[m.Name] = t
	return nil
}

func (r *ToolRegistry) Get(name string) (Tool, error) {
	t, exists := r.tools[name]
	if !exists {
		return nil, errors.New("tool not found: " + name)
	}
	return t, nil
}

func (r *ToolRegistry) Manifests() []ToolManifest {
	manifests := make([]ToolManifest, 0, len(r.tools))
	for _, t := range r.tools {
		manifests = append(manifests, t.Manifest())
	}
	return manifests
}
