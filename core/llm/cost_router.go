package llm

import (
	"context"
	"errors"
	"sort"
)

// CostRouter selects the cheapest healthy provider that meets a minimum capability threshold.
type CostRouter struct {
	providers     []Provider
	minCapability int
}

func NewCostRouter(providers []Provider, minCapability int) *CostRouter {
	return &CostRouter{
		providers:     providers,
		minCapability: minCapability,
	}
}

func (r *CostRouter) Select(ctx context.Context, task string) (Provider, error) {
	//nolint:prealloc // prealloc false positive
	var candidates []Provider
	for _, p := range r.providers {
		if p.Status() != StatusHealthy {
			continue
		}
		if p.Metadata().CapabilityRank < r.minCapability {
			continue
		}
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		return nil, errors.New("no providers meet the health and capability requirements")
	}

	// Sort candidates by cost (ascending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Metadata().CostPer1kTokens < candidates[j].Metadata().CostPer1kTokens
	})

	return candidates[0], nil
}
