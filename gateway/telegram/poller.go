package telegram

import (
	"context"
	"log/slog"
	"time"
)

const (
	// pollTimeoutSeconds is the Telegram server-side long-poll window.
	// Must be ≤ 50 per Telegram API docs.
	pollTimeoutSeconds = 30

	// backoffBase is the initial wait before retrying after a poll error.
	backoffBase = 2 * time.Second

	// backoffMax caps the exponential back-off to avoid very long gaps.
	backoffMax = 60 * time.Second
)

// UpdateHandler is called by the Poller for every incoming Telegram update.
// Implementations must be non-blocking or spawn their own goroutines.
type UpdateHandler func(ctx context.Context, upd Update)

// Poller runs a long-polling loop against the Telegram getUpdates API.
// It tracks the update offset automatically and applies exponential back-off
// on transient errors.  It shuts down cleanly when ctx is cancelled.
type Poller struct {
	client  *Client
	handler UpdateHandler
	log     *slog.Logger
}

// NewPoller creates a Poller using the given client.
// handler is invoked once per incoming Update in a dedicated goroutine.
func NewPoller(client *Client, handler UpdateHandler, log *slog.Logger) *Poller {
	return &Poller{client: client, handler: handler, log: log}
}

// Run blocks, polling Telegram for updates until ctx is cancelled.
// It is safe to call Run exactly once per Poller instance.
func (p *Poller) Run(ctx context.Context) {
	var (
		offset  int64
		backoff = backoffBase
	)

	p.log.Info("telegram_poller_started", slog.Int("poll_timeout_s", pollTimeoutSeconds))

	for {
		select {
		case <-ctx.Done():
			p.log.Info("telegram_poller_stopped")
			return
		default:
		}

		updates, err := p.client.GetUpdates(ctx, offset, pollTimeoutSeconds)
		if err != nil {
			// Context cancellation is a clean shutdown, not an error.
			if ctx.Err() != nil {
				p.log.Info("telegram_poller_stopped")
				return
			}
			p.log.Error("telegram_poll_error",
				slog.String("error", err.Error()),
				slog.Duration("retry_in", backoff),
			)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			backoff = minDuration(backoff*2, backoffMax)
			continue
		}

		// Successful poll — reset backoff.
		backoff = backoffBase

		for _, upd := range updates {
			offset = upd.UpdateID + 1
			// Dispatch each update in its own goroutine so one slow handler
			// cannot delay subsequent updates.
			go p.handler(ctx, upd)
		}
	}
}

// minDuration returns the smaller of a and b (backoff cap helper).
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
