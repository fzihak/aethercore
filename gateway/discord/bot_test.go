package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fzihak/aethercore/sdk"
)

func TestBot_Start_EmptyToken(t *testing.T) {
	bot := NewBot("", sdk.NewModuleRegistry())
	err := bot.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
	if !strings.Contains(err.Error(), "bot token must not be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBot_Start_GatewayURLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	originalBase := discordAPIBase
	discordAPIBase = srv.URL
	defer func() { discordAPIBase = originalBase }()

	bot := NewBot("bad-token", sdk.NewModuleRegistry())
	err := bot.Start(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "token validation failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBot_Start_Success(t *testing.T) {
	srv := makeServer(t, map[string]any{"url": "wss://localhost:12345", "shards": 1})
	defer srv.Close()

	originalBase := discordAPIBase
	discordAPIBase = srv.URL
	defer func() { discordAPIBase = originalBase }()

	bot := NewBot("good-token", sdk.NewModuleRegistry())

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- bot.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errCh
	if err != nil {
		t.Fatalf("Start returned unexpected error: %v", err)
	}
}
