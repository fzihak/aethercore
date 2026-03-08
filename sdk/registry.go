package sdk

import (
	"errors"
	"fmt"
	"sync"
)


// Sentinel errors for the ModuleRegistry.
var (
	ErrNilModule           = errors.New("sdk: cannot register nil module")
	ErrModuleAlreadyLoaded = errors.New("sdk: module already loaded")
	ErrModuleNotFound      = errors.New("sdk: module not found")
	ErrNilModuleTool       = errors.New("sdk: cannot register nil tool")
)

// ModuleToolAlreadyRegisteredError is returned when two tools share the same name.
type ModuleToolAlreadyRegisteredError struct {
	Name string
}

func (e *ModuleToolAlreadyRegisteredError) Error() string {
	return "sdk: tool already registered: " + e.Name
}

// ModuleRegistry is the kernel-side catalogue of loaded Layer 1 Modules.
// All operations are safe for concurrent use from multiple goroutines.
type ModuleRegistry struct {
	mu      sync.RWMutex
	modules map[string]Module
}

// NewModuleRegistry allocates an empty ModuleRegistry.
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{modules: make(map[string]Module)}
}

// Load registers a module into the registry and invokes its OnStart lifecycle hook.
// Returns ErrNilModule if m is nil, ErrModuleAlreadyLoaded if the name is taken.
func (r *ModuleRegistry) Load(m Module, mc *ModuleContext) error {
	if m == nil {
		return ErrNilModule
	}

	manifest := m.Manifest()
	name := manifest.Name

	r.mu.Lock()
	if _, exists := r.modules[name]; exists {
		r.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrModuleAlreadyLoaded, name)
	}
	r.modules[name] = m
	r.mu.Unlock()

	mc.log.Info("module_loading",
		"module", name,
		"version", manifest.Version,
		"author", manifest.Author,
	)
	return nil
}

// Unload removes a module from the registry. It does NOT call OnStop;
// callers are responsible for invoking OnStop before calling Unload.
func (r *ModuleRegistry) Unload(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.modules[name]; !exists {
		return fmt.Errorf("%w: %s", ErrModuleNotFound, name)
	}
	delete(r.modules, name)
	return nil
}

// Get returns the Module registered under name, or ErrModuleNotFound.
func (r *ModuleRegistry) Get(name string) (Module, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.modules[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrModuleNotFound, name)
	}
	return m, nil
}

// Manifests returns a snapshot of all loaded module manifests.
func (r *ModuleRegistry) Manifests() []ModuleManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ModuleManifest, 0, len(r.modules))
	for _, m := range r.modules {
		out = append(out, m.Manifest())
	}
	return out
}

// Len returns the number of loaded modules.
func (r *ModuleRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.modules)
}

// NewModuleContext creates a ModuleContext for the given module name.
// The kernel calls this before passing the context to Module.OnStart.
// Exposed here so the SDK remains self-contained for testing.
func NewModuleContext(moduleName string) *ModuleContext {
	return newModuleContext(moduleName)
}
