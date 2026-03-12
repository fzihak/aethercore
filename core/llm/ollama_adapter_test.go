package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ollamaTextResponse builds a canned Ollama /api/chat response with plain text content.
func ollamaTextResponse(t *testing.T, model, content string) []byte {
	t.Helper()
	resp := ollamaChatResponse{
		Model:      model,
		Message:    ollamaMessage{Role: "assistant", Content: content},
		Done:       true,
		PromptEval: 10,
		EvalCount:  20,
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("ollamaTextResponse: marshal: %v", err)
	}
	return raw
}

// ollamaToolCallResponse builds a canned response where the model requests a tool.
func ollamaToolCallResponse(t *testing.T, model, toolName string, args map[string]any) []byte {
	t.Helper()
	rawArgs, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("ollamaToolCallResponse: marshal args: %v", err)
	}
	resp := ollamaChatResponse{
		Model: model,
		Message: ollamaMessage{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ollamaToolCall{
				{Function: ollamaToolCallFunc{
					Name:      toolName,
					Arguments: rawArgs,
				}},
			},
		},
		Done:       true,
		PromptEval: 15,
		EvalCount:  5,
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("ollamaToolCallResponse: marshal: %v", err)
	}
	return raw
}

// makeOllamaServer creates an httptest.Server that serves the given body for every request.
func makeOllamaServer(t *testing.T, body []byte, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write(body) //nolint:errcheck
	}))
}

// ---- GenerateWithTools tests ------------------------------------------------

func TestOllamaAdapter_textResponse(t *testing.T) {
	body := ollamaTextResponse(t, "llama3.2", "The answer is 42.")
	srv := makeOllamaServer(t, body, http.StatusOK)
	defer srv.Close()

	adapter := newTestOllamaAdapter("llama3.2", srv.URL)
	res, err := adapter.GenerateWithTools(context.Background(), []Message{
		{Role: "user", Content: "What is the answer?"},
	}, nil)
	if err != nil {
		t.Fatalf("GenerateWithTools: %v", err)
	}
	if res.Content != "The answer is 42." {
		t.Errorf("want content=%q, got %q", "The answer is 42.", res.Content)
	}
	if len(res.ToolCalls) != 0 {
		t.Errorf("expected no tool calls on text response, got %d", len(res.ToolCalls))
	}
}

func TestOllamaAdapter_toolCallResponse(t *testing.T) {
	body := ollamaToolCallResponse(t, "llama3.2", "sys_info", map[string]any{})
	srv := makeOllamaServer(t, body, http.StatusOK)
	defer srv.Close()

	adapter := newTestOllamaAdapter("llama3.2", srv.URL)
	tools := []ToolManifest{{
		Name:        "sys_info",
		Description: "Returns current system metrics.",
	}}

	res, err := adapter.GenerateWithTools(context.Background(), []Message{
		{Role: "user", Content: "Show me system info"},
	}, tools)
	if err != nil {
		t.Fatalf("GenerateWithTools: %v", err)
	}
	if len(res.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(res.ToolCalls))
	}
	if res.ToolCalls[0].Name != "sys_info" {
		t.Errorf("want tool name=sys_info, got %q", res.ToolCalls[0].Name)
	}
	// Verify synthesised ID is non-empty
	if res.ToolCalls[0].ID == "" {
		t.Error("expected non-empty synthesised ToolCall.ID")
	}
}

func TestOllamaAdapter_tokenUsage(t *testing.T) {
	body := ollamaTextResponse(t, "llama3.2", "ok")
	srv := makeOllamaServer(t, body, http.StatusOK)
	defer srv.Close()

	adapter := newTestOllamaAdapter("llama3.2", srv.URL)
	res, err := adapter.GenerateWithTools(context.Background(), []Message{
		{Role: "user", Content: "ping"},
	}, nil)
	if err != nil {
		t.Fatalf("GenerateWithTools: %v", err)
	}
	if res.TokenUsage.PromptTokens != 10 {
		t.Errorf("want 10 prompt tokens, got %d", res.TokenUsage.PromptTokens)
	}
	if res.TokenUsage.CompletionTokens != 20 {
		t.Errorf("want 20 completion tokens, got %d", res.TokenUsage.CompletionTokens)
	}
	if res.TokenUsage.TotalTokens != 30 {
		t.Errorf("want 30 total tokens, got %d", res.TokenUsage.TotalTokens)
	}
}

func TestOllamaAdapter_httpError_returnsError(t *testing.T) {
	srv := makeOllamaServer(t, []byte(`{"error":"model not found"}`), http.StatusNotFound)
	defer srv.Close()

	adapter := newTestOllamaAdapter("bad-model", srv.URL)
	_, err := adapter.GenerateWithTools(context.Background(), []Message{
		{Role: "user", Content: "hello"},
	}, nil)
	if err == nil {
		t.Fatal("expected error on HTTP 404, got nil")
	}
}

func TestOllamaAdapter_Name(t *testing.T) {
	a := NewOllamaAdapter("mistral")
	if a.Name() != "ollama/mistral" {
		t.Errorf("want name=ollama/mistral, got %q", a.Name())
	}
}

func TestOllamaAdapter_Generate_delegatesToGenerateWithTools(t *testing.T) {
	body := ollamaTextResponse(t, "llama3.2", "hello from ollama")
	srv := makeOllamaServer(t, body, http.StatusOK)
	defer srv.Close()

	adapter := newTestOllamaAdapter("llama3.2", srv.URL)
	out, err := adapter.Generate(context.Background(), "You are helpful.", "Say hello.")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "hello from ollama" {
		t.Errorf("want %q, got %q", "hello from ollama", out)
	}
}

// ---- toOllamaTool tests -----------------------------------------------------

func TestToOllamaTool_validParameters(t *testing.T) {
	params := json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`)
	tm := ToolManifest{Name: "search", Description: "Web search", Parameters: params}
	got := toOllamaTool(tm)
	if got.Type != "function" {
		t.Errorf("want type=function, got %q", got.Type)
	}
	if got.Function.Name != "search" {
		t.Errorf("want function.name=search, got %q", got.Function.Name)
	}
}

func TestToOllamaTool_nilParameters_usesDefault(t *testing.T) {
	tm := ToolManifest{Name: "noop", Description: "No-op"}
	got := toOllamaTool(tm)
	if !json.Valid(got.Function.Parameters) {
		t.Error("expected valid JSON for nil parameters fallback")
	}
}

// ---- toOllamaMessage tests --------------------------------------------------

func TestToOllamaMessage_toolRole_singleResult(t *testing.T) {
	adapter := newTestOllamaAdapter("x", "http://localhost")
	m := Message{
		Role: "tool",
		ToolResults: []ToolResultMessage{
			{ToolCallID: "c1", Content: "42 bytes free", IsError: false},
		},
	}
	om := adapter.toOllamaMessage(m)
	if om.Content != "42 bytes free" {
		t.Errorf("want content=%q, got %q", "42 bytes free", om.Content)
	}
}

func TestToOllamaMessage_assistantWithToolCalls(t *testing.T) {
	adapter := newTestOllamaAdapter("x", "http://localhost")
	m := Message{
		Role:    "assistant",
		Content: "",
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "sys_info", Arguments: `{}`},
		},
	}
	om := adapter.toOllamaMessage(m)
	if len(om.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(om.ToolCalls))
	}
	if om.ToolCalls[0].Function.Name != "sys_info" {
		t.Errorf("want tool name=sys_info, got %q", om.ToolCalls[0].Function.Name)
	}
}

func TestOllamaAdapter_contextCancellation(t *testing.T) {
	// Server that blocks — context should time it out
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // block until client cuts the connection
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	adapter := newTestOllamaAdapter("llama3.2", srv.URL)
	_, err := adapter.GenerateWithTools(ctx, []Message{{Role: "user", Content: "hello"}}, nil)
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
}
