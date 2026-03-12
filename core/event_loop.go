package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/fzihak/aethercore/core/audit"
	"github.com/fzihak/aethercore/core/llm"
	"github.com/fzihak/aethercore/core/security"
)

// Inviolable Rule: Layer 0 strictly uses Go stdlib ONLY.

var (
	ErrQueueFull  = errors.New("worker queue is full")
	ErrTaskFailed = errors.New("task execution failed")
)

// Task represents a single unit of work in AetherCore.
// Ephemeral agents are instantiated per task.
type Task struct {
	ID        string
	System    string
	Input     string
	CreatedAt time.Time
}

// Result encapsulates the outcome of a Task.
type Result struct {
	TaskID   string
	Output   string
	Duration time.Duration
	Error    error
}

// Engine coordinates the worker pool and ephemeral execution lifecycle.
type Engine struct {
	adapter       llm.LLMAdapter
	tools         *ToolRegistry
	sandboxClient *SandboxClient // optional: Layer 2 Rust Sandbox for unknown tools
	taskQueue     chan *Task
	resultQueue   chan *Result
	workerCount   int
	wg            sync.WaitGroup
	quit          chan struct{}
	stopOnce      sync.Once
	taskPool      sync.Pool
	resultPool    sync.Pool
	guard         security.PromptGuard
	audit         audit.Logger
}

// NewEngine initializes the core event loop with bounded goroutines.
func NewEngine(adapter llm.LLMAdapter, workerCount, queueSize int) *Engine {
	e := &Engine{
		adapter:     adapter,
		tools:       NewToolRegistry(nil),
		taskQueue:   make(chan *Task, queueSize),
		resultQueue: make(chan *Result, queueSize),
		workerCount: workerCount,
		quit:        make(chan struct{}),
		guard: security.NewOrchestratorGuard(
			security.NewRegexScanner(),
			security.NewSemanticAnalyzer(),
			security.NewLLMVerifier(adapter),
		),
	}
	e.taskPool.New = func() interface{} {
		return &Task{}
	}
	e.resultPool.New = func() interface{} {
		return &Result{}
	}
	return e
}

// WithSandbox attaches a Layer 2 Rust Sandbox client to this Engine.
// After this call, any tool call whose name is absent from the local registry
// is automatically forwarded to the verified Rust sidecar for execution.
func (e *Engine) WithSandbox(client *SandboxClient) *Engine {
	e.sandboxClient = client
	return e
}

// RegisterTool adds a tool to the engine's ephemeral registry.
func (e *Engine) RegisterTool(t Tool) error {
	return e.tools.Register(t)
}

// WithAuditLogger attaches the cryptographic audit sidecar to record all activities immutably.
func (e *Engine) WithAuditLogger(l audit.Logger) *Engine {
	e.audit = l
	return e
}

// Start boots the worker pool. Sub-50ms target for Pico Mode.
func (e *Engine) Start() {
	if e.audit != nil {
		_ = e.audit.LogEvent(context.Background(), &audit.Event{
			ID:        "sys-boot",
			Timestamp: time.Now(),
			Type:      "AUDIT_ENGINE_BOOT",
			Actor:     "system",
			Metadata:  map[string]interface{}{"worker_count": e.workerCount},
		})
	}
	for i := range e.workerCount {
		e.wg.Add(1)
		go e.worker(i)
	}
}

// Stop gracefully shuts down the worker pool.
func (e *Engine) Stop() {
	e.stopOnce.Do(func() {
		// Signal all workers to terminate their loops
		close(e.quit)

		// Strictly block until every single ephemeral worker has returned
		e.wg.Wait()

		// Only after all workers are dead is it safe to close the queues
		close(e.taskQueue)
		close(e.resultQueue)
	})
}

// GetTask retrieves a zero-allocated Task from the sync pool.
func (e *Engine) GetTask() *Task {
	t, ok := e.taskPool.Get().(*Task)
	if !ok {
		return &Task{}
	}
	return t
}

// GetResult retrieves a zero-allocated Result from the sync pool.
func (e *Engine) GetResult() *Result {
	r, ok := e.resultPool.Get().(*Result)
	if !ok {
		return &Result{}
	}
	return r
}

// Submit enqueues a task. Returns ErrQueueFull if the bounded queue is saturated.
func (e *Engine) Submit(t *Task) error {
	select {
	case e.taskQueue <- t:
		return nil
	default:
		return ErrQueueFull
	}
}

// Results provides a read-only channel to consume task outcomes.
func (e *Engine) Results() <-chan *Result {
	return e.resultQueue
}

// RecycleResult securely scrubs and returns the Result pointer to the sync pool.
func (e *Engine) RecycleResult(r *Result) {
	if r == nil {
		return
	}
	r.TaskID = ""
	r.Output = ""
	r.Error = nil
	r.Duration = 0
	e.resultPool.Put(r)
}

// worker is the ephemeral execution primitive. State is strictly scoped to the task.
func (e *Engine) worker(id int) {
	defer e.wg.Done()
	for {
		select {
		case <-e.quit:
			return
		case t := <-e.taskQueue:
			start := time.Now()

			taskLog := WithTask(context.Background(), t.ID).With(slog.Int("worker_id", id))
			taskLog.Info("ephemeral_task_started")

			out, err := e.executeEphemeral(t)
			duration := time.Since(start)

			if err != nil {
				taskLog.Error("ephemeral_task_failed", slog.String("error", err.Error()), slog.Duration("duration_ms", duration))
			} else {
				taskLog.Info("ephemeral_task_completed", slog.Duration("duration_ms", duration))
			}

			res := e.GetResult()
			res.TaskID = t.ID
			res.Output = out
			res.Duration = duration
			res.Error = err

			e.resultQueue <- res

			// Recycle the pointer back into the pool. Zero allocations.
			t.ID = ""
			t.System = ""
			t.Input = ""
			e.taskPool.Put(t)
		}
	}
}

const maxAgentIterations = 10

// executeEphemeral is the core orchestration loop for a single task.
// No state leaks outside this function.
func (e *Engine) executeEphemeral(t *Task) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var messages []llm.Message
	messages = append(messages, llm.Message{Role: "system", Content: "You are AetherCore Kernel. Execute the objective using tools."})
	messages = append(messages, llm.Message{Role: "user", Content: t.Input})

	guardRes := e.guard.Scan(ctx, t.Input, security.GuardConfig{})
	if !guardRes.IsSafe {
		WithTask(ctx, t.ID).Warn("security_violation_user_input",
			slog.String("rule", guardRes.Violations[0].Category),
			slog.String("description", guardRes.Violations[0].Description),
		)
		return "", fmt.Errorf("security_violation: %s", guardRes.Violations[0].Description)
	}

	for iteration := range maxAgentIterations {
		if e.audit != nil {
			_ = e.audit.LogEvent(ctx, &audit.Event{
				ID:        t.ID + "-req",
				Timestamp: time.Now(),
				Type:      "AUDIT_LLM_REQUEST",
				Actor:     "engine",
				Metadata:  map[string]interface{}{"task_id": t.ID, "messages_count": len(messages)},
			})
		}

		res, err := e.adapter.GenerateWithTools(ctx, messages, e.tools.Manifests())
		if err != nil {
			return "", fmt.Errorf("llm_iter_%d: %w", iteration, err)
		}

		// LLM decided it's done — no more tool calls
		if len(res.ToolCalls) == 0 {
			return res.Content, nil
		}

		// Append assistant turn to history
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   res.Content,
			ToolCalls: res.ToolCalls,
		})

		// Execute tools, feed results back
		var results []llm.ToolResultMessage
		for _, call := range res.ToolCalls {
			result, execErr := e.dispatchTool(ctx, t.ID, call)

			var contentStr string
			if execErr != nil {
				if strings.Contains(execErr.Error(), "security_violation") {
					return "", execErr
				}
				contentStr = execErr.Error()
			} else {
				contentStr = result
			}

			results = append(results, llm.ToolResultMessage{
				ToolCallID: call.ID,
				Content:    contentStr,
				IsError:    execErr != nil,
			})
		}

		messages = append(messages, llm.Message{
			Role:        "tool",
			ToolResults: results,
		})
	}

	return "", errors.New("ErrMaxIterationsExceeded")
}

// dispatchTool dynamically resolves execution to Layer 0 (internal) or Layer 2 (sandbox).
func (e *Engine) dispatchTool(ctx context.Context, taskID string, call llm.ToolCall) (string, error) {
	tool, err := e.tools.Get(call.Name)
	if err == nil {
		toolLog := WithComponent("tool_executor").With(slog.String("tool_name", call.Name))
		toolLog.Debug("tool_execution_started", slog.String("arguments", call.Arguments))
		if e.audit != nil {
			_ = e.audit.LogEvent(ctx, &audit.Event{
				ID:        taskID + "-tool-exec",
				Timestamp: time.Now(),
				Type:      "AUDIT_TOOL_EXECUTE",
				Actor:     "engine",
				Metadata:  map[string]interface{}{"task_id": taskID, "tool": call.Name},
			})
		}
		toolStart := time.Now()

		res, execErr := tool.Execute(ctx, call.Arguments)
		toolDuration := time.Since(toolStart)

		if execErr == nil {
			if scanErr := e.verifyToolOutput(ctx, taskID, call.Name, res); scanErr != nil {
				return "", scanErr
			}
		}

		if execErr != nil {
			toolLog.Error("tool_execution_failed", slog.String("error", execErr.Error()), slog.Duration("duration_ms", toolDuration))
			return "", execErr
		}
		toolLog.Info("tool_execution_completed", slog.Duration("duration_ms", toolDuration))
		return res, nil
	}

	// Unknown tool — forward to Rust sandbox
	if e.sandboxClient != nil {
		sbLog := WithComponent("sandbox_dispatcher").With(slog.String("tool_name", call.Name))
		sbLog.Info("unknown_tool_dispatched_to_sandbox")

		sbStart := time.Now()
		output, sbErr := e.sandboxClient.ExecuteTool(ctx, call.Name, call.Arguments, "")
		sbDuration := time.Since(sbStart)

		if e.audit != nil {
			_ = e.audit.LogEvent(ctx, &audit.Event{
				ID:        taskID + "-tool-res",
				Timestamp: time.Now(),
				Type:      "AUDIT_TOOL_RESULT",
				Actor:     "engine",
				Metadata:  map[string]interface{}{"task_id": taskID, "tool_name": call.Name, "duration_ms": sbDuration.Milliseconds()},
			})
		}

		if sbErr != nil {
			sbLog.Error("sandbox_execution_failed",
				slog.String("error", sbErr.Error()),
				slog.Duration("duration_ms", sbDuration),
			)
			return "", sbErr
		}

		if scanErr := e.verifyToolOutput(ctx, taskID, call.Name, output); scanErr != nil {
			return "", scanErr
		}

		sbLog.Info("sandbox_execution_completed", slog.Duration("duration_ms", sbDuration))
		return output, nil
	}

	return "", fmt.Errorf("tool_not_found: %s", call.Name)
}

func (e *Engine) verifyToolOutput(ctx context.Context, taskID, toolName, output string) error {
	if e.guard == nil {
		return nil
	}

	res := e.guard.Scan(ctx, output, security.GuardConfig{})
	if res.IsSafe {
		return nil
	}

	if e.audit != nil {
		_ = e.audit.LogEvent(ctx, &audit.Event{
			ID:        taskID + "-violation",
			Timestamp: time.Now(),
			Type:      "AUDIT_SECURITY_VIOLATION",
			Actor:     "prompt-guard",
			Metadata:  map[string]interface{}{"task_id": taskID, "reason": res.Violations[0].Description},
		})
	}

	WithComponent("tool_executor").Warn("tool_output_security_violation_detected",
		slog.String("tool", toolName),
		slog.String("rule", res.Violations[0].Category),
		slog.String("description", res.Violations[0].Description),
	)
	return fmt.Errorf("security_violation_tool_output: %s", res.Violations[0].Description)
}
