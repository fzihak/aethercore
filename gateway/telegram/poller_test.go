package telegram

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// noopLogger returns a logger that discards all output, useful for tests.
func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(new(noopWriter), &slog.HandlerOptions{Level: slog.LevelError}))
}

type noopWriter struct{}

func (n *noopWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func TestPoller_ContextCancellation(t *testing.T) {
	// A server that hangs until the test finishes to ensure we don't return early
	// and trigger un-cancellable paths.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second) // Wait longer than the context
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient("token", srv.URL+"/bot")
	poller := NewPoller(client, func(ctx context.Context, upd Update) {}, noopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := mockPollerRun(poller, ctx)

	select {
	case <-done:
		// Success, poller exited
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Poller did not exit on context cancellation")
	}
}

func TestPoller_SuccessfulUpdates(t *testing.T) {
	payload := []map[string]any{
		{
			"update_id": 100,
			"message": map[string]any{
				"message_id": 1, "date": 0, "text": "/help",
				"chat": map[string]any{"id": 5, "type": "private"},
			},
		},
		{
			"update_id": 101,
			"message": map[string]any{
				"message_id": 2, "date": 0, "text": "/start",
				"chat": map[string]any{"id": 5, "type": "private"},
			},
		},
	}

	var mu sync.Mutex
	requestCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		count := requestCount
		mu.Unlock()

		if count > 1 {
			// On subsequent requests, just return empty to not keep triggering handlers
			emptyResp := okResponse([]Update{})
			raw, _ := json.Marshal(emptyResp)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
			return
		}

		// First request, return the payload
		raw, err := json.Marshal(okResponse(payload))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	client := newTestClient("token", srv.URL+"/bot")

	var receivedUpdates []Update
	var handlerMu sync.Mutex
	updatesDone := make(chan struct{})

	handler := func(ctx context.Context, upd Update) {
		handlerMu.Lock()
		receivedUpdates = append(receivedUpdates, upd)
		if len(receivedUpdates) == 2 {
			close(updatesDone)
		}
		handlerMu.Unlock()
	}

	poller := NewPoller(client, handler, noopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := mockPollerRun(poller, ctx)

	// Wait for the updates to be processed deterministically
	select {
	case <-updatesDone:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for updates to be processed")
	}

	cancel()

	<-done

	handlerMu.Lock()
	defer handlerMu.Unlock()

	if len(receivedUpdates) != 2 {
		t.Fatalf("want 2 updates, got %d", len(receivedUpdates))
	}
	if receivedUpdates[0].UpdateID != 100 {
		t.Errorf("want UpdateID 100, got %d", receivedUpdates[0].UpdateID)
	}
	if receivedUpdates[1].UpdateID != 101 {
		t.Errorf("want UpdateID 101, got %d", receivedUpdates[1].UpdateID)
	}
}

func TestPoller_ErrorBackoff(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		// Always return an error to trigger backoff
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newTestClient("token", srv.URL+"/bot")
	poller := NewPoller(client, func(ctx context.Context, upd Update) {}, noopLogger())

	ctx, cancel := context.WithCancel(context.Background())

	// Start poller in a goroutine
	done := mockPollerRun(poller, ctx)

	// Wait slightly longer than the first backoff (backoffBase is 2s, but we'll mock it
	// ideally, but since we can't easily override constants, we just let it run and
	// wait 50ms, then cancel. We want to ensure it handles context cancellation during backoff.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success, poller exited during backoff or shortly after
	case <-time.After(3 * time.Second):
		t.Fatal("Poller did not exit on context cancellation during backoff")
	}

	mu.Lock()
	count := requestCount
	mu.Unlock()

	if count == 0 {
		t.Errorf("expected at least one request, got %d", count)
	}
}

func TestMinDuration(t *testing.T) {
	d1 := 1 * time.Second
	d2 := 2 * time.Second

	if minDuration(d1, d2) != d1 {
		t.Errorf("expected %v, got %v", d1, minDuration(d1, d2))
	}
	if minDuration(d2, d1) != d1 {
		t.Errorf("expected %v, got %v", d1, minDuration(d2, d1))
	}
}

func mockPollerRun(p *Poller, ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()
	return done
}
