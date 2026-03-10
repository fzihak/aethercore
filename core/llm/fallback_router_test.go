package llm

import (
	"context"
	"testing"
)

type MockProvider struct {
	name     string
	status   Status
	priority Priority
}

func (m *MockProvider) Name() string            { return m.name }
func (m *MockProvider) Status() Status          { return m.status }
func (m *MockProvider) Priority() Priority      { return m.priority }
func (m *MockProvider) Metadata() ModelMetadata { return ModelMetadata{} }
func (m *MockProvider) Execute(ctx context.Context, task string) (string, error) {
	return "mock-result", nil
}

func TestFallbackRouter_Select(t *testing.T) {
	t.Run("selects highest priority healthy provider", func(t *testing.T) {
		p1 := &MockProvider{name: "primary", status: StatusHealthy, priority: 1}
		p2 := &MockProvider{name: "secondary", status: StatusHealthy, priority: 2}

		router := NewFallbackRouter([]Provider{p1, p2})
		got, err := router.Select(context.Background(), "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name() != "primary" {
			t.Errorf("expected primary, got %s", got.Name())
		}
	})

	t.Run("falls back to secondary if primary is offline", func(t *testing.T) {
		p1 := &MockProvider{name: "primary", status: StatusOffline, priority: 1}
		p2 := &MockProvider{name: "secondary", status: StatusHealthy, priority: 2}

		router := NewFallbackRouter([]Provider{p1, p2})
		got, err := router.Select(context.Background(), "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name() != "secondary" {
			t.Errorf("expected secondary, got %s", got.Name())
		}
	})

	t.Run("returns error if no providers are healthy", func(t *testing.T) {
		p1 := &MockProvider{name: "primary", status: StatusOffline, priority: 1}

		router := NewFallbackRouter([]Provider{p1})
		_, err := router.Select(context.Background(), "test")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
