# AetherCore API Documentation

AetherCore provides a minimal, strict API interface for building Autonomous Agents within the Layer 0 (and eventually Layer 1/2) environment.

## 1. Engine Core

**Package:** `github.com/fzihak/aethercore/core`

### `core.SubmitTask(ctx context.Context, task *sys.Task)`

Asynchronously dispatches a generic task onto the Event Loop for `TaskExecutor` resolution. Returns a channel that delivers the `Result`.

```go
resCh := core.SubmitTask(ctx, &sys.Task{ ID: "xyz", Goal: "Compute sequence" })
res := <-resCh
```

_Note:_ Tasks are immediately rejected if their payload exceeds the configurable Kernel Maximum Task Size parameter to prevent OOM saturation attacks.

### `core.Logger()`

Returns a zero-allocation `*slog.Logger` instance pointing to the deterministic JSON OpenTelemetry stream configured upon Kernel startup. Output goes to `os.Stdout`.

## 2. Tools

**Package:** `github.com/fzihak/aethercore/core/tools`

Every tool in AetherCore implements the `core.Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, args []byte) ([]byte, error)
}
```

Currently implemented builtin Layer 0 Tools:

- `SysInfoTool`: Exposes OS metadata (Memory, CPU, Hostname).

## 3. SDK & Modules (Layer 1)

**Package:** `github.com/fzihak/aethercore/sdk`

Layer 1 capabilities are packaged as autonomous Modules, tracked in the `ModuleRegistry`.

### `sdk.Module` Interface

```go
type Module interface {
    Manifest() ModuleManifest
    OnStart(context.Context, *ModuleContext) error
    OnStop(context.Context) error
    HandleTask(context.Context, *ModuleTask) (*ModuleResult, error)
}
```

### Module Lifecycle

Modules are explicitly loaded by the orchestrator at startup.
All Modules run their `OnStart()` hooks asynchronously. If a Module panics, the Kernel intercepts it and logs a fatal crash to standard error without crashing peer modules.

### The Scheduler Module

**Package:** `github.com/fzihak/aethercore/modules/scheduler`
Cron parser handling task orchestration with pure Go implementation (`O(1)` bitset evaluation).

- `AddJob(name, cron, goal string) error`
- `EnableJob(name)`
- `RemoveJob(name)`

## 4. Telemetry

AetherCore guarantees minimal log output under extreme concurrency by dumping only deterministic JSON chunks to stdout. Ensure all external monitors tail this stdout using standard Unix pipelines.
