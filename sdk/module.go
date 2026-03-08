// Package sdk is the public AetherCore Layer 1 Module SDK.
//
// Developers import this package to build AetherCore modules — self-contained
// units of capability that plug into the kernel's ephemeral execution loop.
// Each module declares its identity via [ModuleManifest] and implements the
// [Module] interface to receive task dispatches from the Go Kernel.
//
// Layer 1 Modules differ from Layer 0 Tools in scope:
//   - Tools are fine-grained, single-function executors (e.g. sys_info).
//   - Modules are higher-level, stateful-but-ephemeral participants that can
//     register one or more tools, subscribe to lifecycle events, and coordinate
//     across the mesh via the [ModuleContext].
//
// Usage:
//
//	type MyModule struct{}
//
//	func (m *MyModule) Manifest() sdk.ModuleManifest { ... }
//	func (m *MyModule) OnStart(ctx context.Context, mc *sdk.ModuleContext) error { ... }
//	func (m *MyModule) OnStop(ctx context.Context) error { ... }
//	func (m *MyModule) HandleTask(ctx context.Context, t *sdk.ModuleTask) (*sdk.ModuleResult, error) { ... }
package sdk

import "context"

// Module is the interface every Layer 1 Module must satisfy.
// The kernel calls OnStart on registration, HandleTask for each dispatched
// task, and OnStop during graceful shutdown.
type Module interface {
	// Manifest returns the static metadata and capability declaration for this module.
	Manifest() ModuleManifest

	// OnStart is called once when the module is loaded into the kernel.
	// Use it to initialise resources, register sub-tools, or connect to upstream services.
	// The provided ModuleContext gives access to kernel services (logger, tool registry, etc).
	OnStart(ctx context.Context, mc *ModuleContext) error

	// OnStop is called once during graceful kernel shutdown.
	// Release all held resources here; the context carries the shutdown deadline.
	OnStop(ctx context.Context) error

	// HandleTask processes a single task dispatched to this module by the kernel.
	// Implementations must be safe for concurrent calls from multiple goroutines.
	HandleTask(ctx context.Context, task *ModuleTask) (*ModuleResult, error)
}

// ModuleManifest is the declarative identity envelope attached to every module.
// The kernel enforces the stated capabilities before the module may execute.
type ModuleManifest struct {
	// Name is the unique, kebab-case identifier for the module (e.g. "web-search").
	Name string `json:"name"`

	// Description is a one-line human-readable summary of the module's purpose.
	Description string `json:"description"`

	// Version follows Semantic Versioning (e.g. "1.0.0").
	Version string `json:"version"`

	// Author is the module developer's name or organisation.
	Author string `json:"author"`

	// Capabilities lists the kernel permissions this module requires.
	// The kernel refuses to load the module if undeclared capabilities are exercised.
	Capabilities []Capability `json:"capabilities"`

	// MaxTaskRuntimeMs is the hard deadline for a single HandleTask call in milliseconds.
	// The kernel injects a context deadline derived from this value.
	MaxTaskRuntimeMs int `json:"max_task_runtime_ms"`

	// MemoryLimitMB is the advisory memory ceiling for this module in megabytes.
	// Enforced by the Rust Sandbox (Layer 2) when the module is sandboxed.
	MemoryLimitMB int `json:"memory_limit_mb"`
}

// Capability declares a permission the module requires from the kernel.
// Matches the Capability type enforced in the Layer 0 ToolRegistry.
type Capability string

const (
	// CapNetwork permits outbound TCP/UDP connections.
	CapNetwork Capability = "network"

	// CapFilesystem permits reads and writes to the local filesystem.
	CapFilesystem Capability = "filesystem"

	// CapState permits reading and writing the AetherCore vector memory store.
	CapState Capability = "state"

	// CapMesh permits propagating tasks to remote AetherCore nodes via the mesh.
	CapMesh Capability = "mesh"
)

// ModuleTask is the unit of work handed to a module by the kernel dispatcher.
type ModuleTask struct {
	// ID is the globally unique task identifier (matches the core.Task.ID).
	ID string `json:"id"`

	// Input is the raw user goal or LLM-constructed instruction for the module.
	Input string `json:"input"`

	// Metadata carries optional key-value pairs injected by the kernel or other modules.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ModuleResult is the structured output produced by [Module.HandleTask].
type ModuleResult struct {
	// TaskID echoes the originating ModuleTask.ID.
	TaskID string `json:"task_id"`

	// Output is the text or JSON payload produced by the module.
	Output string `json:"output"`

	// Metadata carries optional key-value annotations for downstream consumers.
	Metadata map[string]string `json:"metadata,omitempty"`
}
