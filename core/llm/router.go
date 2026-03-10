package llm

import (
	"context"
)

// Router defines the logic for selecting the most appropriate LLM provider
// based on task complexity, cost, and availability.
type Router interface {
	// Select returns the best LLM provider for the given context.
	Select(ctx context.Context, task string) (Provider, error)
}

// Status represents the health of an LLM provider.
type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusOffline  Status = "offline"
)

// ModelMetadata provides heuristics about cost and performance.
type ModelMetadata struct {
	CostPer1kTokens float64 // in USD
	LatencyMillis   int     // expected average latency
	CapabilityRank  int     // 1-10, where 10 is high-reasoning (e.g. GPT-4)
}

// Priority represents the selection rank (lower is higher priority).
type Priority int

// Provider is a specialized interface that the Router works with.
// It abstracts the underlying LLMAdapter to add routing-specific metadata.
type Provider interface {
	Name() string
	Status() Status
	Priority() Priority
	Metadata() ModelMetadata
	Execute(ctx context.Context, task string) (string, error)
}
