package llm

import (
	"context"
	"encoding/json"
)

// Capability flags define what a tool is permitted to do.
type Capability string

const (
	CapNetwork    Capability = "network"
	CapFilesystem Capability = "filesystem"
	CapState      Capability = "state"
)

// LLMAdapter defines the contract for any LLM provider used by the kernel.
type LLMAdapter interface {
	Generate(ctx context.Context, systemPrompt, userInput string) (string, error)
	GenerateWithTools(ctx context.Context, messages []Message, tools []ToolManifest) (LLMResponse, error)
	Name() string
}

// LLMResponse encapsulates the response from the LLM, including tool invocations if any.
type LLMResponse struct {
	Content    string
	ToolCalls  []ToolCall
	TokenUsage TokenUsage
}

// Message represents a single turn in a conversational ReAct loop history.
type Message struct {
	Role        string // "system", "user", "assistant", "tool"
	Content     string
	ToolCalls   []ToolCall
	ToolResults []ToolResultMessage
}

// ToolResultMessage holds the feedback from an executed local or sandboxed tool.
type ToolResultMessage struct {
	ToolCallID string
	Content    string
	IsError    bool
}

// ToolCall represents a deterministic request from the LLM to execute a tool.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// TokenUsage tracks the resource utilization per request.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ToolManifest defines the declarative capability-envelope for a tool.
type ToolManifest struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Parameters   json.RawMessage `json:"parameters"`
	Capabilities []Capability    `json:"capabilities"`
	MaxRuntimeMs int             `json:"max_runtime_ms"`
	MemoryLimit  int             `json:"memory_limit_mb"`
}
