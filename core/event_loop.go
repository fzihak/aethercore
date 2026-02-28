package core

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
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
	adapter     LLMAdapter
	tools       *ToolRegistry
	taskQueue   chan Task
	resultQueue chan Result
	workerCount int
	wg          sync.WaitGroup
	quit        chan struct{}
}

// NewEngine initializes the core event loop with bounded goroutines.
func NewEngine(adapter LLMAdapter, workerCount int, queueSize int) *Engine {
	return &Engine{
		adapter:     adapter,
		tools:       NewToolRegistry(),
		taskQueue:   make(chan Task, queueSize),
		resultQueue: make(chan Result, queueSize),
		workerCount: workerCount,
		quit:        make(chan struct{}),
	}
}

// RegisterTool adds a tool to the engine's ephemeral registry.
func (e *Engine) RegisterTool(t Tool) error {
	return e.tools.Register(t)
}

// Start boots the worker pool. Sub-50ms target for Pico Mode.
func (e *Engine) Start() {
	for i := 0; i < e.workerCount; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}
}

// Stop gracefully shuts down the worker pool.
func (e *Engine) Stop() {
	// Signal all workers to terminate their loops
	close(e.quit)

	// Strictly block until every single ephemeral worker has returned
	e.wg.Wait()

	// Only after all workers are dead is it safe to close the queues
	close(e.taskQueue)
	close(e.resultQueue)
}

// Submit enqueues a task. Returns ErrQueueFull if the bounded queue is saturated.
func (e *Engine) Submit(t Task) error {
	select {
	case e.taskQueue <- t:
		return nil
	default:
		return ErrQueueFull
	}
}

// Results provides a read-only channel to consume task outcomes.
func (e *Engine) Results() <-chan Result {
	return e.resultQueue
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

			e.resultQueue <- Result{
				TaskID:   t.ID,
				Output:   out,
				Duration: duration,
				Error:    err,
			}
		}
	}
}

// executeEphemeral is the core orchestration loop for a single task.
// No state leaks outside this function.
func (e *Engine) executeEphemeral(t Task) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	manifests := e.tools.Manifests()

	// 1. Send prompt to LLM
	resp, err := e.adapter.GenerateWithTools(ctx, t.System, t.Input, manifests)
	if err != nil {
		return "", err
	}

	// 2. If no tool calls, return purely text response
	if len(resp.ToolCalls) == 0 {
		return resp.Content, nil
	}

	// 3. Process Deterministic Tool Calls
	// In Layer 0, we only handle in-process capability execution directly.
	// Untrusted calls will be delegated to the Layer 2 Rust Sandbox via RPC later.
	for _, call := range resp.ToolCalls {
		tool, err := e.tools.Get(call.Name)
		if err != nil {
			WithComponent("tool_orchestrator").Warn("tool_not_found_in_registry", slog.String("tool_name", call.Name))
			continue
		}

		// capability enforcement will wrap `Execute` here
		toolLog := WithComponent("tool_executor").With(slog.String("tool_name", call.Name))
		toolLog.Debug("tool_execution_started", slog.String("arguments", call.Arguments))

		toolStart := time.Now()
		_, err = tool.Execute(ctx, call.Arguments)
		toolDuration := time.Since(toolStart)

		if err != nil {
			toolLog.Error("tool_execution_failed", slog.String("error", err.Error()), slog.Duration("duration_ms", toolDuration))
			continue
		}

		toolLog.Info("tool_execution_completed", slog.Duration("duration_ms", toolDuration))

		// Note: A real LLM loop would feed the result back to the LLM here.
		// For Layer 0 scaffolding, we just execute sequentially.
	}

	return resp.Content, nil
}
