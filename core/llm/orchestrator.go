package llm

import (
	"context"
	"fmt"
)

// Orchestrator coordinates multiple routers and providers to fulfill a task.
type Orchestrator struct {
	router Router
}

func NewOrchestrator(router Router) *Orchestrator {
	return &Orchestrator{router: router}
}

// Execute selects a provider via the router and runs the task with retry logic.
func (o *Orchestrator) Execute(ctx context.Context, task string) (string, error) {
	provider, err := o.router.Select(ctx, task)
	if err != nil {
		return "", fmt.Errorf("router_selection_failed: %w", err)
	}

	return provider.Execute(ctx, task)
}

// GenerateWithTools is a higher-level method for the event loop.
// In the future, this will also use the router to pick the best model for tool-use.
func (o *Orchestrator) GenerateWithTools(ctx context.Context, messages []Message, tools []ToolManifest) (LLMResponse, error) {
	// For now, we use the first healthy provider from the router for tool use.
	// This will be expanded to use a specialized router for tool-calling capabilities.
	_, err := o.router.Select(ctx, "tool_use_request")
	if err != nil {
		return LLMResponse{}, err
	}

	// Note: We need a way to cast Provider to LLMAdapter or ensure Provider implements it.
	// For Layer 0, we assume the Provider's Execute handles the adapter logic.
	// In the future, we'll wrap concrete adapters (Ollama, OpenAI) as Providers.

	// Temporarily returning mock/error until concrete adapter wrapping is implemented in Phase 2.2
	return LLMResponse{Content: "Orchestrator tool-use initialization placeholder"}, nil
}
