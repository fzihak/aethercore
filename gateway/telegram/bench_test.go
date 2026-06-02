package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fzihak/aethercore/sdk"
)

// noopServer builds a test server that intercepts sendMessage calls.
func noopServer(tb testing.TB) *httptest.Server {
	tb.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always respond with a valid Telegram envelope so the client doesn't error.
		resp := okResponse(map[string]any{
			"message_id": 1,
			"chat":       map[string]any{"id": 1, "type": "private"},
			"date":       0,
			"text":       "",
		})
		raw, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
}

// echoModuleBench is a minimal sdk.Module that echoes its task input.
type echoModuleBench struct{ name string }

func (e *echoModuleBench) Manifest() sdk.ModuleManifest {
	return sdk.ModuleManifest{
		Name:             e.name,
		Description:      "Echoes input",
		Version:          "1.0.0",
		Author:           "test",
		MaxTaskRuntimeMs: 1000,
	}
}
func (e *echoModuleBench) OnStart(_ context.Context, _ *sdk.ModuleContext) error { return nil }
func (e *echoModuleBench) OnStop(_ context.Context) error                        { return nil }
func (e *echoModuleBench) HandleTask(_ context.Context, t *sdk.ModuleTask) (*sdk.ModuleResult, error) {
	return &sdk.ModuleResult{TaskID: t.ID, Output: "echo:" + t.Input}, nil
}

func BenchmarkAdapterHandleRun(b *testing.B) {
	srv := noopServer(b)
	defer srv.Close()

	registry := sdk.NewModuleRegistry()
	for i := range 100 {
		name := fmt.Sprintf("echo-%d", i)
		mod := &echoModuleBench{name: name}
		mc := sdk.NewModuleContext(name)
		_ = sdk.StartModule(context.Background(), registry, mod, mc)
	}

	adapter := newTestAdapter(srv, registry)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		adapter.HandleRun(ctx, 1, "hello")
	}
}
