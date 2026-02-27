package core

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// MockLLMAdapter provides a dummy LLM for testing
type MockLLMAdapter struct{}

func (m *MockLLMAdapter) Generate(ctx context.Context, systemPrompt, userInput string) (string, error) {
	return "Mock Response", nil
}

func (m *MockLLMAdapter) GenerateWithTools(ctx context.Context, systemPrompt, userInput string, tools []ToolManifest) (LLMResponse, error) {
	// Dummy response for event loop test
	return LLMResponse{
		Content: "Mock Content with Tools",
	}, nil
}

func (m *MockLLMAdapter) Name() string {
	return "Mock"
}

func TestEventLoopWorkerLimits(t *testing.T) {
	adapter := &MockLLMAdapter{}
	// Engine with 2 workers
	engine := NewEngine(adapter, 2, 100)

	engine.Start()

	// Enqueue 5 tasks
	for i := 0; i < 5; i++ {
		err := engine.Submit(Task{
			ID:        "t",
			System:    "Sys",
			Input:     "Input",
			CreatedAt: time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
	}

	// Collect 5 results
	for i := 0; i < 5; i++ {
		res := <-engine.Results()
		if res.Error != nil {
			t.Errorf("Task failed: %v", res.Error)
		}
		if res.Output != "Mock Content with Tools" {
			t.Errorf("Unexpected output: %s", res.Output)
		}
	}

	engine.Stop()
}

func TestEventLoopGoroutineLeak(t *testing.T) {
	// Baseline
	initialGoroutines := runtime.NumGoroutine()

	adapter := &MockLLMAdapter{}
	engine := NewEngine(adapter, 10, 1000)

	engine.Start()

	// Flood the worker pool
	const taskCount = 100
	for i := 0; i < taskCount; i++ {
		err := engine.Submit(Task{
			ID:        "leak_test",
			System:    "Sys",
			Input:     "Input",
			CreatedAt: time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
	}

	// Drain results
	for i := 0; i < taskCount; i++ {
		<-engine.Results()
	}

	engine.Stop()

	// Give the runtime a tiny buffer to sweep dead goroutines before failing
	time.Sleep(10 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()

	if finalGoroutines > initialGoroutines {
		t.Fatalf("Goroutine leak detected! Initial: %d, Final: %d", initialGoroutines, finalGoroutines)
	}
}
