package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fzihak/aethercore/sdk"
)

func TestBot_Start(t *testing.T) {
	registry := sdk.NewModuleRegistry()

	t.Run("EmptyToken", func(t *testing.T) {
		bot := NewBot("", registry)
		err := bot.Start(context.Background())
		if err == nil {
			t.Fatal("expected error for empty token, got nil")
		}
		if !strings.Contains(err.Error(), "must not be empty") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("GetMeFailure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/getMe") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
				return
			}
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}))
		defer srv.Close()

		bot := NewBot("fake:token", registry)
		bot.log = newTestLogger()
		bot.client = newTestClient("fake:token", srv.URL)

		err := bot.Start(context.Background())
		if err == nil {
			t.Fatal("expected error on GetMe failure, got nil")
		}
		if !strings.Contains(err.Error(), "token validation failed") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/getMe") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"ok":true,"result":{"id":12345,"is_bot":true,"first_name":"TestBot","username":"testbot"}}`))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/getUpdates") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"ok":true,"result":[]}`))
				return
			}
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}))
		defer srv.Close()

		bot := NewBot("fake:token", registry)
		bot.log = newTestLogger()
		bot.client = newTestClient("fake:token", srv.URL)

		// Use a context that will cancel shortly so Start doesn't block forever
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := bot.Start(ctx)
		if err != nil {
			t.Fatalf("expected no error on success, got: %v", err)
		}
	})
}
