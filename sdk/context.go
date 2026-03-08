package sdk

import (
	"context"
	"log/slog"
	"os"
)

// ModuleContext is the handle the kernel passes to [Module.OnStart].
// It provides access to every kernel service a Layer 1 Module is permitted
// to use — logging, tool registration, and (if CapMesh is declared) the mesh.
// All methods on ModuleContext are safe for concurrent use.
type ModuleContext struct {
	moduleName string
	log        *slog.Logger
	tools      *ModuleToolRegistry
}

// newModuleContext builds a context bound to the given module name.
// The kernel calls this internally; SDK users receive it via OnStart.
func newModuleContext(moduleName string) *ModuleContext {
	opts := &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
			}
			if a.Key == slog.MessageKey {
				a.Key = "msg"
			}
			return a
		},
	}
	base := slog.New(slog.NewJSONHandler(os.Stdout, opts)).With(
		slog.String("service.name", "aethercore"),
		slog.String("component", "module."+moduleName),
	)
	return &ModuleContext{
		moduleName: moduleName,
		log:        base,
		tools:      newModuleToolRegistry(),
	}
}

// Logger returns a structured JSON logger pre-tagged with the module name.
// Use this for all module-internal log output to stay consistent with
// AetherCore's OpenTelemetry JSON format.
func (mc *ModuleContext) Logger() *slog.Logger {
	return mc.log
}

// RegisterTool adds a [ModuleTool] to the kernel's tool registry on behalf of
// this module. The tool will be discoverable via 'aether tool list' and callable
// from the LLM tool-calling loop once the module is loaded.
// Returns an error if a tool with the same name is already registered.
func (mc *ModuleContext) RegisterTool(t ModuleTool) error {
	return mc.tools.register(t)
}

// Tools returns the names of all tools registered by this module so far.
func (mc *ModuleContext) Tools() []string {
	return mc.tools.names()
}

// ModuleTool is the lightweight interface modules use to expose callable tools
// to the AetherCore LLM loop. It mirrors core.Tool but lives in the SDK so
// that Layer 1 Modules have zero dependency on the internal kernel package.
type ModuleTool interface {
	// ToolName returns the unique kebab-case identifier (e.g. "web-search").
	ToolName() string

	// ToolDescription is the one-line summary shown in 'aether tool list'.
	ToolDescription() string

	// Run executes the tool with the given JSON argument string and returns
	// a JSON-serialisable result string. ctx carries the task deadline.
	Run(ctx context.Context, argsJSON string) (string, error)
}

// ModuleToolRegistry is a thread-safe store of ModuleTools belonging to one module.
type ModuleToolRegistry struct {
	items map[string]ModuleTool
}

func newModuleToolRegistry() *ModuleToolRegistry {
	return &ModuleToolRegistry{items: make(map[string]ModuleTool)}
}

func (r *ModuleToolRegistry) register(t ModuleTool) error {
	if t == nil {
		return ErrNilModuleTool
	}
	name := t.ToolName()
	if _, exists := r.items[name]; exists {
		return &ModuleToolAlreadyRegisteredError{Name: name}
	}
	r.items[name] = t
	return nil
}

func (r *ModuleToolRegistry) names() []string {
	out := make([]string, 0, len(r.items))
	for k := range r.items {
		out = append(out, k)
	}
	return out
}
