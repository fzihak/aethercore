package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaAdapter implements LLMAdapter against a locally running Ollama instance.
// It uses the /api/chat endpoint (Ollama v0.3+) with native tool-call support,
// enabling true multi-iteration ReAct loops without any LangChain dependency.
//
// API reference: https://github.com/ollama/ollama/blob/main/docs/api.md#chat
type OllamaAdapter struct {
	model   string
	baseURL string // overridable for tests via newTestOllamaAdapter
	http    *http.Client
}

// NewOllamaAdapter constructs an adapter pointing at the default local Ollama server.
func NewOllamaAdapter(model string) *OllamaAdapter {
	return &OllamaAdapter{
		model:   model,
		baseURL: "http://localhost:11434",
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

// NewOllamaAdapterWithURL constructs an adapter pointing at the provided base URL.
// Intended for integration tests and non-default Ollama deployments.
func NewOllamaAdapterWithURL(model, baseURL string) *OllamaAdapter {
	return &OllamaAdapter{
		model:   model,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// newTestOllamaAdapter constructs an adapter pointing at a test server (e.g. httptest.Server).
func newTestOllamaAdapter(model, baseURL string) *OllamaAdapter {
	return &OllamaAdapter{
		model:   model,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// Name returns the adapter identifier used in routing tables.
func (a *OllamaAdapter) Name() string { return "ollama/" + a.model }

// Generate is a convenience wrapper for single-turn text generation.
func (a *OllamaAdapter) Generate(ctx context.Context, systemPrompt, userInput string) (string, error) {
	msgs := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userInput},
	}
	res, err := a.GenerateWithTools(ctx, msgs, nil)
	if err != nil {
		return "", err
	}
	return res.Content, nil
}

// GenerateWithTools sends a chat-completion request to Ollama with tool definitions.
// When the model decides to invoke a tool it returns a non-empty ToolCalls slice;
// the Engine's ReAct loop appends the tool result and calls again.
func (a *OllamaAdapter) GenerateWithTools(ctx context.Context, messages []Message, tools []ToolManifest) (LLMResponse, error) {
	reqBody := ollamaChatRequest{
		Model:  a.model,
		Stream: false,
	}

	for _, m := range messages {
		reqBody.Messages = append(reqBody.Messages, a.toOllamaMessage(m))
	}

	for _, t := range tools {
		reqBody.Tools = append(reqBody.Tools, toOllamaTool(t))
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("ollama: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("ollama: POST /api/chat: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("ollama: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return LLMResponse{}, fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, raw)
	}

	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(raw, &ollamaResp); err != nil {
		return LLMResponse{}, fmt.Errorf("ollama: decode response: %w", err)
	}

	return a.fromOllamaResponse(ollamaResp), nil
}

// ---- Ollama wire types -------------------------------------------------------

// ollamaChatRequest is the JSON body for POST /api/chat.
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
}

// ollamaMessage is a single turn in the Ollama chat history.
type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

// ollamaToolCall matches the format Ollama returns when the model requests a tool.
type ollamaToolCall struct {
	Function ollamaToolCallFunc `json:"function"`
}

type ollamaToolCallFunc struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ollamaTool describes a tool in the Ollama tool-definition format.
type ollamaTool struct {
	Type     string         `json:"type"` // always "function"
	Function ollamaFunction `json:"function"`
}

type ollamaFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ollamaChatResponse is the top-level JSON envelope returned by Ollama.
type ollamaChatResponse struct {
	Model      string        `json:"model"`
	Message    ollamaMessage `json:"message"`
	Done       bool          `json:"done"`
	PromptEval int           `json:"prompt_eval_count"`
	EvalCount  int           `json:"eval_count"`
}

// ---- conversion helpers -----------------------------------------------------

// toOllamaMessage converts an engine-internal Message to the Ollama wire format.
// Role "tool" (ToolResults) is expanded: one Ollama message per tool result,
// using role "tool" and the content string.
func (a *OllamaAdapter) toOllamaMessage(m Message) ollamaMessage {
	switch m.Role {
	case "assistant":
		om := ollamaMessage{Role: "assistant", Content: m.Content}
		for i, tc := range m.ToolCalls {
			args := json.RawMessage(tc.Arguments)
			if !json.Valid(args) {
				args = json.RawMessage(`{}`)
			}
			om.ToolCalls = append(om.ToolCalls, ollamaToolCall{
				Function: ollamaToolCallFunc{
					Name:      tc.Name,
					Arguments: args,
				},
			})
			_ = i // suppress unused warning
		}
		return om
	default:
		// "system", "user", "tool" — Ollama accepts all with plain content
		content := m.Content
		if m.Role == "tool" && len(m.ToolResults) > 0 {
			// Flatten the first result; multi-result is serialised as JSON array
			if len(m.ToolResults) == 1 {
				content = m.ToolResults[0].Content
			} else {
				raw, _ := json.Marshal(m.ToolResults)
				content = string(raw)
			}
		}
		return ollamaMessage{Role: m.Role, Content: content}
	}
}

// fromOllamaResponse maps an Ollama API response back to the engine-internal LLMResponse.
func (a *OllamaAdapter) fromOllamaResponse(r ollamaChatResponse) LLMResponse {
	res := LLMResponse{
		Content: r.Message.Content,
		TokenUsage: TokenUsage{
			PromptTokens:     r.PromptEval,
			CompletionTokens: r.EvalCount,
			TotalTokens:      r.PromptEval + r.EvalCount,
		},
	}

	for i, tc := range r.Message.ToolCalls {
		// Ollama doesn't return a tool call ID — synthesise one so the engine
		// can match results back to requests in multi-tool batches.
		id := fmt.Sprintf("call_%s_%d", tc.Function.Name, i)
		res.ToolCalls = append(res.ToolCalls, ToolCall{
			ID:        id,
			Name:      tc.Function.Name,
			Arguments: string(tc.Function.Arguments),
		})
	}

	return res
}

// toOllamaTool converts an engine-internal ToolManifest to the Ollama wire format.
func toOllamaTool(t ToolManifest) ollamaTool {
	params := t.Parameters
	if len(params) == 0 || !json.Valid(params) {
		params = json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
	}
	return ollamaTool{
		Type: "function",
		Function: ollamaFunction{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		},
	}
}
