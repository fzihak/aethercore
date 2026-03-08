package sdk

import (
	"context"
	"fmt"
	"time"
)

const (
	// defaultStartDeadlineMs is used when a module does not declare MaxTaskRuntimeMs.
	defaultStartDeadlineMs = 5_000
	// defaultStopDeadlineMs is the shutdown deadline applied to OnStop calls.
	defaultStopDeadlineMs = 3_000
)

// StartModule calls m.OnStart with a context deadline derived from
// m.Manifest().MaxTaskRuntimeMs (falls back to defaultStartDeadlineMs),
// then registers m in r.
//
// If OnStart returns an error the module is NOT registered and the error
// is returned to the caller.
func StartModule(ctx context.Context, r *ModuleRegistry, m Module, mc *ModuleContext) error {
	if m == nil {
		return ErrNilModule
	}

	manifest := m.Manifest()
	deadlineMs := manifest.MaxTaskRuntimeMs
	if deadlineMs <= 0 {
		deadlineMs = defaultStartDeadlineMs
	}

	startCtx, cancel := context.WithTimeout(ctx, time.Duration(deadlineMs)*time.Millisecond)
	defer cancel()

	if err := m.OnStart(startCtx, mc); err != nil {
		return fmt.Errorf("sdk: module %q OnStart failed: %w", manifest.Name, err)
	}

	return r.Load(m, mc)
}

// StopModule calls m.OnStop with a fixed deadline of defaultStopDeadlineMs,
// then removes the module from the registry.
//
// If the module is not registered, ErrModuleNotFound is returned.
// OnStop errors are wrapped and returned; the module is still unloaded.
func StopModule(ctx context.Context, r *ModuleRegistry, name string) error {
	m, err := r.Get(name)
	if err != nil {
		return err
	}

	stopCtx, cancel := context.WithTimeout(ctx, defaultStopDeadlineMs*time.Millisecond)
	defer cancel()

	stopErr := m.OnStop(stopCtx)
	_ = r.Unload(name) // always unload even if OnStop errs

	if stopErr != nil {
		return fmt.Errorf("sdk: module %q OnStop failed: %w", name, stopErr)
	}
	return nil
}

// StopAll stops every loaded module concurrently applying defaultStopDeadlineMs
// per module.  It waits for all goroutines to finish and returns a joined error
// if any OnStop call failed.
//
// Modules are unloaded from r regardless of OnStop outcome.
func StopAll(ctx context.Context, r *ModuleRegistry) error {
	manifests := r.Manifests()
	if len(manifests) == 0 {
		return nil
	}

	type result struct {
		name string
		err  error
	}
	ch := make(chan result, len(manifests))

	for _, mf := range manifests {
		name := mf.Name
		go func() {
			ch <- result{name: name, err: StopModule(ctx, r, name)}
		}()
	}

	var errs []error
	for range manifests {
		if res := <-ch; res.err != nil {
			errs = append(errs, res.err)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("sdk: StopAll encountered %d error(s): %w", len(errs), errs[0])
}
