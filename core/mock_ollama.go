package core

import (
	"context"
	"strings"

	"github.com/fzihak/aethercore/core/llm"
)

// MockOllamaAdapter simulates an LLM for testing the core orchestration loops natively.
// It bypasses the network entirely to ensure deterministic execution paths.
type MockOllamaAdapter struct{}

// NewMockOllamaAdapter initializes a zero-allocation mock engine.
func NewMockOllamaAdapter() *MockOllamaAdapter {
	return &MockOllamaAdapter{}
}

// Generate implements a basic static text generation response.
func (m *MockOllamaAdapter) Generate(ctx context.Context, systemPrompt, userInput string) (string, error) {
	return "Mock response completed.", nil
}

// GenerateWithTools simulates an LLM intercepting a prompt and deciding to trigger a specific tool.
func (m *MockOllamaAdapter) GenerateWithTools(ctx context.Context, messages []llm.Message, tools []llm.ToolManifest) (llm.LLMResponse, error) {
	// Simple mock logic: if the user explicitly asks for system info in the last message, forcefully return the ToolCall
	var lastUserInput string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserInput = messages[i].Content
			break
		}
	}

	if strings.Contains(strings.ToLower(lastUserInput), "system info") {
		return llm.LLMResponse{
			Content: "",
			ToolCalls: []llm.ToolCall{
				{
					ID:        "call_mock123",
					Name:      "sys_info",
					Arguments: "{}", // sys_info doesn't require complex JSON arguments
				},
			},
			TokenUsage: llm.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}, nil
	}

	// Default fallback to standard text generation if no trigger phrase is matched
	return llm.LLMResponse{
		Content: "I am a mocked intelligence. I did not detect any tool triggers.",
		TokenUsage: llm.TokenUsage{
			PromptTokens:     5,
			CompletionTokens: 15,
			TotalTokens:      20,
		},
	}, nil
}

// Name identifies this adapter to the Kernel routing tables.
func (m *MockOllamaAdapter) Name() string {
	return "mock_ollama"
}
