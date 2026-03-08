package sdk_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/fzihak/aethercore/sdk"
)

// ---- test double -------------------------------------------------------

type fakeModule struct {
	manifest sdk.ModuleManifest
	startErr error
	stopErr  error

	started bool
	stopped bool
}

func (m *fakeModule) Manifest() sdk.ModuleManifest { return m.manifest }

func (m *fakeModule) OnStart(_ context.Context, _ *sdk.ModuleContext) error {
	m.started = true
	return m.startErr
}

func (m *fakeModule) OnStop(_ context.Context) error {
	m.stopped = true
	return m.stopErr
}

func (m *fakeModule) HandleTask(_ context.Context, t *sdk.ModuleTask) (*sdk.ModuleResult, error) {
	return &sdk.ModuleResult{TaskID: t.ID, Output: "ok"}, nil
}

func newFake(name string) *fakeModule {
	return &fakeModule{
		manifest: sdk.ModuleManifest{
			Name:             name,
			Version:          "1.0.0",
			Author:           "test",
			MaxTaskRuntimeMs: 500,
		},
	}
}

// ---- ModuleRegistry ----------------------------------------------------

func TestNewModuleRegistry_isEmpty(t *testing.T) {
	r := sdk.NewModuleRegistry()
	if r.Len() != 0 {
		t.Fatalf("expected empty registry, got Len=%d", r.Len())
	}
}

func TestLoad_success(t *testing.T) {
	r := sdk.NewModuleRegistry()
	mc := sdk.NewModuleContext("alpha")

	if err := r.Load(newFake("alpha"), mc); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.Len() != 1 {
		t.Fatalf("want Len=1, got %d", r.Len())
	}
}

func TestLoad_nilModule(t *testing.T) {
	r := sdk.NewModuleRegistry()
	mc := sdk.NewModuleContext("x")

	if err := r.Load(nil, mc); !errors.Is(err, sdk.ErrNilModule) {
		t.Fatalf("want ErrNilModule, got %v", err)
	}
}

func TestLoad_duplicateName(t *testing.T) {
	r := sdk.NewModuleRegistry()
	mc := sdk.NewModuleContext("alpha")

	_ = r.Load(newFake("alpha"), mc)
	err := r.Load(newFake("alpha"), sdk.NewModuleContext("alpha"))

	if !errors.Is(err, sdk.ErrModuleAlreadyLoaded) {
		t.Fatalf("want ErrModuleAlreadyLoaded, got %v", err)
	}
}

func TestUnload_success(t *testing.T) {
	r := sdk.NewModuleRegistry()
	mc := sdk.NewModuleContext("beta")
	_ = r.Load(newFake("beta"), mc)

	if err := r.Unload("beta"); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	if r.Len() != 0 {
		t.Fatalf("expected empty registry after Unload, got Len=%d", r.Len())
	}
}

func TestUnload_notFound(t *testing.T) {
	r := sdk.NewModuleRegistry()
	if err := r.Unload("ghost"); !errors.Is(err, sdk.ErrModuleNotFound) {
		t.Fatalf("want ErrModuleNotFound, got %v", err)
	}
}

func TestGet_success(t *testing.T) {
	r := sdk.NewModuleRegistry()
	mc := sdk.NewModuleContext("gamma")
	_ = r.Load(newFake("gamma"), mc)

	m, err := r.Get("gamma")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if m.Manifest().Name != "gamma" {
		t.Fatalf("want name=gamma, got %q", m.Manifest().Name)
	}
}

func TestGet_notFound(t *testing.T) {
	r := sdk.NewModuleRegistry()
	if _, err := r.Get("ghost"); !errors.Is(err, sdk.ErrModuleNotFound) {
		t.Fatalf("want ErrModuleNotFound, got %v", err)
	}
}

func TestManifests_snapshot(t *testing.T) {
	r := sdk.NewModuleRegistry()
	for _, name := range []string{"a", "b", "c"} {
		_ = r.Load(newFake(name), sdk.NewModuleContext(name))
	}

	manifests := r.Manifests()
	if len(manifests) != 3 {
		t.Fatalf("want 3 manifests, got %d", len(manifests))
	}
}

func TestRegistry_concurrentLoads_raceDetector(t *testing.T) {
	r := sdk.NewModuleRegistry()

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("mod-%d", i)
			_ = r.Load(newFake(name), sdk.NewModuleContext(name))
		}(i)
	}
	wg.Wait()

	if r.Len() != n {
		t.Fatalf("want Len=%d after concurrent loads, got %d", n, r.Len())
	}
}

// ---- ModuleContext / tool registry -------------------------------------

func TestModuleContext_registerTool(t *testing.T) {
	mc := sdk.NewModuleContext("ctx-test")

	tool := &fakeTool{name: "ping"}
	if err := mc.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool: %v", err)
	}

	names := mc.Tools()
	if len(names) != 1 || names[0] != "ping" {
		t.Fatalf("unexpected tools: %v", names)
	}
}

func TestModuleContext_registerDuplicateTool(t *testing.T) {
	mc := sdk.NewModuleContext("ctx-dup")
	_ = mc.RegisterTool(&fakeTool{name: "ping"})

	err := mc.RegisterTool(&fakeTool{name: "ping"})
	if err == nil {
		t.Fatal("expected error registering duplicate tool, got nil")
	}
}

func TestModuleContext_registerNilTool(t *testing.T) {
	mc := sdk.NewModuleContext("ctx-nil")
	if err := mc.RegisterTool(nil); !errors.Is(err, sdk.ErrNilModuleTool) {
		t.Fatalf("want ErrNilModuleTool, got %v", err)
	}
}

// ---- StartModule / StopModule / StopAll --------------------------------

func TestStartModule_callsOnStart(t *testing.T) {
	r := sdk.NewModuleRegistry()
	mod := newFake("start-test")
	mc := sdk.NewModuleContext("start-test")

	if err := sdk.StartModule(context.Background(), r, mod, mc); err != nil {
		t.Fatalf("StartModule: %v", err)
	}
	if !mod.started {
		t.Fatal("expected OnStart to be called")
	}
	if r.Len() != 1 {
		t.Fatalf("expected module registered after StartModule, got Len=%d", r.Len())
	}
}

func TestStartModule_onStartError_doesNotRegister(t *testing.T) {
	r := sdk.NewModuleRegistry()
	mod := newFake("fail-start")
	mod.startErr = errors.New("boom")
	mc := sdk.NewModuleContext("fail-start")

	if err := sdk.StartModule(context.Background(), r, mod, mc); err == nil {
		t.Fatal("expected error from StartModule, got nil")
	}
	if r.Len() != 0 {
		t.Fatalf("module should NOT be registered on OnStart error, Len=%d", r.Len())
	}
}

func TestStartModule_nilModule(t *testing.T) {
	r := sdk.NewModuleRegistry()
	err := sdk.StartModule(context.Background(), r, nil, sdk.NewModuleContext("x"))
	if !errors.Is(err, sdk.ErrNilModule) {
		t.Fatalf("want ErrNilModule, got %v", err)
	}
}

func TestStopModule_callsOnStop(t *testing.T) {
	r := sdk.NewModuleRegistry()
	mod := newFake("stop-test")
	mc := sdk.NewModuleContext("stop-test")
	_ = sdk.StartModule(context.Background(), r, mod, mc)

	if err := sdk.StopModule(context.Background(), r, "stop-test"); err != nil {
		t.Fatalf("StopModule: %v", err)
	}
	if !mod.stopped {
		t.Fatal("expected OnStop to be called")
	}
	if r.Len() != 0 {
		t.Fatalf("module should be unloaded after StopModule, Len=%d", r.Len())
	}
}

func TestStopModule_notFound(t *testing.T) {
	r := sdk.NewModuleRegistry()
	if err := sdk.StopModule(context.Background(), r, "ghost"); !errors.Is(err, sdk.ErrModuleNotFound) {
		t.Fatalf("want ErrModuleNotFound, got %v", err)
	}
}

func TestStopAll_stopsAllModules(t *testing.T) {
	r := sdk.NewModuleRegistry()
	mods := make([]*fakeModule, 5)
	for i := range mods {
		mods[i] = newFake(fmt.Sprintf("m%d", i))
		mc := sdk.NewModuleContext(fmt.Sprintf("m%d", i))
		_ = sdk.StartModule(context.Background(), r, mods[i], mc)
	}

	if err := sdk.StopAll(context.Background(), r); err != nil {
		t.Fatalf("StopAll: %v", err)
	}
	if r.Len() != 0 {
		t.Fatalf("expected empty registry after StopAll, Len=%d", r.Len())
	}
	for _, m := range mods {
		if !m.stopped {
			t.Fatalf("module %q was not stopped", m.manifest.Name)
		}
	}
}

func TestStopAll_emptyRegistry(t *testing.T) {
	r := sdk.NewModuleRegistry()
	if err := sdk.StopAll(context.Background(), r); err != nil {
		t.Fatalf("StopAll on empty registry: %v", err)
	}
}

// ---- helpers -----------------------------------------------------------

type fakeTool struct {
	name string
}

func (f *fakeTool) ToolName() string        { return f.name }
func (f *fakeTool) ToolDescription() string { return "fake tool" }
func (f *fakeTool) Run(_ context.Context, _ string) (string, error) {
	return "fake", nil
}
