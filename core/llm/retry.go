package llm

import (
	"context"
	"math"
	"time"
)

// RetryingProvider wraps a Provider and adds retry logic with exponential backoff.
type RetryingProvider struct {
	base       Provider
	maxRetries int
}

func NewRetryingProvider(base Provider, maxRetries int) *RetryingProvider {
	return &RetryingProvider{
		base:       base,
		maxRetries: maxRetries,
	}
}

func (p *RetryingProvider) Name() string            { return p.base.Name() }
func (p *RetryingProvider) Status() Status          { return p.base.Status() }
func (p *RetryingProvider) Priority() Priority      { return p.base.Priority() }
func (p *RetryingProvider) Metadata() ModelMetadata { return p.base.Metadata() }

func (p *RetryingProvider) Execute(ctx context.Context, task string) (string, error) {
	var lastErr error
	for i := 0; i < p.maxRetries; i++ {
		res, err := p.base.Execute(ctx, task)
		if err == nil {
			return res, nil
		}
		lastErr = err

		// Skip wait if it's the last attempt
		if i == p.maxRetries-1 {
			break
		}

		// Exponential backoff: 2^i * 100ms (100ms, 200ms, 400ms...)
		backoff := time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "", lastErr
}
