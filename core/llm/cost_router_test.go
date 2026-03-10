package llm

import (
	"context"
	"testing"
)

func TestCostRouter_Select(t *testing.T) {
	t.Run("selects cheapest healthy provider", func(t *testing.T) {
		p1 := &MockProvider{
			name:   "expensive-gpt4",
			status: StatusHealthy,
			Metadata: func() ModelMetadata {
				return ModelMetadata{CostPer1kTokens: 0.03, CapabilityRank: 10}
			}(),
		}
		p2 := &MockProvider{
			name:   "cheap-ollama",
			status: StatusHealthy,
			Metadata: func() ModelMetadata {
				return ModelMetadata{CostPer1kTokens: 0.00, CapabilityRank: 5}
			}(),
		}

		router := NewCostRouter([]Provider{p1, p2}, 1) // Min capability 1
		got, err := router.Select(context.Background(), "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name() != "cheap-ollama" {
			t.Errorf("expected cheap-ollama, got %s", got.Name())
		}
	})

	t.Run("selects expensive if cheap is offline", func(t *testing.T) {
		p1 := &MockProvider{
			name:   "expensive-gpt4",
			status: StatusHealthy,
			Metadata: func() ModelMetadata {
				return ModelMetadata{CostPer1kTokens: 0.03, CapabilityRank: 10}
			}(),
		}
		p2 := &MockProvider{
			name:   "cheap-ollama",
			status: StatusOffline,
			Metadata: func() ModelMetadata {
				return ModelMetadata{CostPer1kTokens: 0.00, CapabilityRank: 5}
			}(),
		}

		router := NewCostRouter([]Provider{p1, p2}, 1)
		got, err := router.Select(context.Background(), "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name() != "expensive-gpt4" {
			t.Errorf("expected expensive-gpt4, got %s", got.Name())
		}
	})

	t.Run("respects capability requirements", func(t *testing.T) {
		p1 := &MockProvider{
			name:   "high-capability",
			status: StatusHealthy,
			Metadata: func() ModelMetadata {
				return ModelMetadata{CostPer1kTokens: 0.10, CapabilityRank: 9}
			}(),
		}
		p2 := &MockProvider{
			name:   "low-capability",
			status: StatusHealthy,
			Metadata: func() ModelMetadata {
				return ModelMetadata{CostPer1kTokens: 0.01, CapabilityRank: 3}
			}(),
		}

		router := NewCostRouter([]Provider{p1, p2}, 7) // Requires 7+
		got, err := router.Select(context.Background(), "high reasoning task")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name() != "high-capability" {
			t.Errorf("expected high-capability, got %s", got.Name())
		}
	})
}
