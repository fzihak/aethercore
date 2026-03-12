package llm

import (
	"context"
	"fmt"
)

// Orchestrator coordinates multiple routers and providers to fulfill a task.
// The optional adapter field provides native tool-call support (GenerateWithTools).
// When adapter is nil, GenerateWithTools falls back to a simple Execute call.
type Orchestrator struct {
	router  Router
	adapter LLMAdapter
}

func NewOrchestrator(router Router) *Orchestrator {
	return &Orchestrator{router: router}
}

// NewOrchestratorWithAdapter constructs an Orchestrator that can execute both
// plain tasks (via router) and multi-turn ReAct tool loops (via adapter).
func NewOrchestratorWithAdapter(router Router, adapter LLMAdapter) *Orchestrator {
	return &Orchestrator{router: router, adapter: adapter}
}

// Execute selects a provider via the router and runs the task with retry logic.
func (o *Orchestrator) Execute(ctx context.Context, task string) (string, error) {
	provider, err := o.router.Select(ctx, task)
	if err != nil {
		return "", fmt.Errorf("router_selection_failed: %w", err)
	}

	return provider.Execute(ctx, task)
}

// GenerateWithTools selects the best provider for tool-use via the router and
// delegates to the configured LLMAdapter. If no adapter is configured, it falls
// back to the router-selected provider's Execute method (no tool parsing).
func (o *Orchestrator) GenerateWithTools(ctx context.Context, messages []Message, tools []ToolManifest) (LLMResponse, error) {
	// If a concrete LLMAdapter is wired in, delegate directly — it owns schema
	// conversion, tool_call parsing, and token-usage accounting.
	if o.adapter != nil {
		return o.adapter.GenerateWithTools(ctx, messages, tools)
	}

	// Fallback: use the router to pick a provider and issue the last user message
	// as a plain text Execute call. This path produces no ToolCalls.
	provider, err := o.router.Select(ctx, "tool_use_request")
	if err != nil {
		return LLMResponse{}, fmt.Errorf("router_selection_failed: %w", err)
	}

	// Extract the last user-role message as the task string.
	task := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			task = messages[i].Content
			break
		}
	}

	content, err := provider.Execute(ctx, task)
	if err != nil {
		return LLMResponse{}, err
	}
	return LLMResponse{Content: content}, nil
}
