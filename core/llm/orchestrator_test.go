package llm

import (
	"context"
	"errors"
	"testing"
)

// ---- stubs ------------------------------------------------------------------

type stubProvider struct {
	name   string
	status Status
	reply  string
	err    error
}

func (s *stubProvider) Name() string           { return s.name }
func (s *stubProvider) Status() Status         { return s.status }
func (s *stubProvider) Priority() Priority     { return 1 }
func (s *stubProvider) Metadata() ModelMetadata { return ModelMetadata{} }
func (s *stubProvider) Execute(_ context.Context, _ string) (string, error) {
	return s.reply, s.err
}

type stubRouter struct {
	p   Provider
	err error
}

func (r *stubRouter) Select(_ context.Context, _ string) (Provider, error) { return r.p, r.err }

type stubAdapter struct {
	res LLMResponse
	err error
}

func (a *stubAdapter) Name() string { return "stub" }
func (a *stubAdapter) Generate(_ context.Context, _, _ string) (string, error) {
	return a.res.Content, a.err
}
func (a *stubAdapter) GenerateWithTools(_ context.Context, _ []Message, _ []ToolManifest) (LLMResponse, error) {
	return a.res, a.err
}

// ---- Execute tests ----------------------------------------------------------

func TestOrchestrator_Execute_success(t *testing.T) {
	router := &stubRouter{p: &stubProvider{name: "p1", status: StatusHealthy, reply: "done"}}
	o := NewOrchestrator(router)
	got, err := o.Execute(context.Background(), "task")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "done" {
		t.Errorf("want=done, got=%q", got)
	}
}

func TestOrchestrator_Execute_routerError(t *testing.T) {
	router := &stubRouter{err: errors.New("no providers")}
	o := NewOrchestrator(router)
	_, err := o.Execute(context.Background(), "task")
	if err == nil {
		t.Fatal("expected error when router fails")
	}
}

// ---- GenerateWithTools with adapter -----------------------------------------

func TestOrchestrator_GenerateWithTools_delegatesToAdapter(t *testing.T) {
	want := LLMResponse{Content: "from adapter", ToolCalls: []ToolCall{{Name: "sys_info"}}}
	adapter := &stubAdapter{res: want}
	o := NewOrchestratorWithAdapter(&stubRouter{p: &stubProvider{status: StatusHealthy}}, adapter)

	got, err := o.GenerateWithTools(context.Background(), []Message{
		{Role: "user", Content: "run sys_info"},
	}, nil)
	if err != nil {
		t.Fatalf("GenerateWithTools: %v", err)
	}
	if got.Content != want.Content {
		t.Errorf("want content=%q, got %q", want.Content, got.Content)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "sys_info" {
		t.Errorf("unexpected tool calls: %v", got.ToolCalls)
	}
}

func TestOrchestrator_GenerateWithTools_adapterError_propagated(t *testing.T) {
	adapter := &stubAdapter{err: errors.New("LLM unavailable")}
	o := NewOrchestratorWithAdapter(&stubRouter{p: &stubProvider{status: StatusHealthy}}, adapter)

	_, err := o.GenerateWithTools(context.Background(), []Message{
		{Role: "user", Content: "hello"},
	}, nil)
	if err == nil {
		t.Fatal("expected error from adapter, got nil")
	}
}

// ---- GenerateWithTools without adapter (fallback path) ----------------------

func TestOrchestrator_GenerateWithTools_fallback_usesLastUserMessage(t *testing.T) {
	router := &stubRouter{p: &stubProvider{status: StatusHealthy, reply: "fallback answer"}}
	o := NewOrchestrator(router) // no adapter

	msgs := []Message{
		{Role: "system", Content: "You are an agent."},
		{Role: "user", Content: "What time is it?"},
	}
	got, err := o.GenerateWithTools(context.Background(), msgs, nil)
	if err != nil {
		t.Fatalf("GenerateWithTools fallback: %v", err)
	}
	if got.Content != "fallback answer" {
		t.Errorf("want content=fallback answer, got %q", got.Content)
	}
	if len(got.ToolCalls) != 0 {
		t.Errorf("fallback path must not return tool calls")
	}
}

func TestOrchestrator_GenerateWithTools_fallback_routerError(t *testing.T) {
	router := &stubRouter{err: errors.New("all offline")}
	o := NewOrchestrator(router)

	_, err := o.GenerateWithTools(context.Background(), []Message{
		{Role: "user", Content: "ping"},
	}, nil)
	if err == nil {
		t.Fatal("expected router error to propagate")
	}
}

func TestNewOrchestratorWithAdapter_nilAdapter_usesRouterFallback(t *testing.T) {
	router := &stubRouter{p: &stubProvider{status: StatusHealthy, reply: "ok"}}
	o := NewOrchestratorWithAdapter(router, nil)
	got, err := o.GenerateWithTools(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Content != "ok" {
		t.Errorf("want ok, got %q", got.Content)
	}
}
