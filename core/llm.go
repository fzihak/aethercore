package core

import (
	"github.com/fzihak/aethercore/core/llm"
)

// LLMAdapter is now a type alias for llm.LLMAdapter to maintain backward compatibility during migration.
// TODO: Eventually remove this once all external callers are updated.
type LLMAdapter = llm.LLMAdapter
type Message = llm.Message
type LLMResponse = llm.LLMResponse
type ToolCall = llm.ToolCall
type ToolResultMessage = llm.ToolResultMessage
type TokenUsage = llm.TokenUsage
type ToolManifest = llm.ToolManifest
