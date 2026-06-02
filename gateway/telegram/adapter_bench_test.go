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

type dynEchoModule struct{ name string }

func (e *dynEchoModule) Manifest() sdk.ModuleManifest {
	return sdk.ModuleManifest{
		Name:             e.name,
		Description:      "Echoes input",
		Version:          "1.0.0",
		Author:           "test",
		MaxTaskRuntimeMs: 1000,
	}
}
func (e *dynEchoModule) OnStart(_ context.Context, _ *sdk.ModuleContext) error { return nil }
func (e *dynEchoModule) OnStop(_ context.Context) error                        { return nil }
func (e *dynEchoModule) HandleTask(_ context.Context, t *sdk.ModuleTask) (*sdk.ModuleResult, error) {
	return &sdk.ModuleResult{TaskID: t.ID, Output: "echo:" + t.Input}, nil
}

func BenchmarkHandleRun(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := okResponse(map[string]any{
			"message_id": 1,
			"chat":       map[string]any{"id": 1, "type": "private"},
			"date":       0,
			"text":       "dummy",
		})
		raw, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	registry := sdk.NewModuleRegistry()
	for i := range 50 {
		name := fmt.Sprintf("echo-%d", i)
		mod := &dynEchoModule{name: name}
		mc := sdk.NewModuleContext(name)
		registry.Load(mod, mc)
	}

	adapter := newTestAdapter(srv, registry)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		adapter.HandleRun(ctx, 1, "hello")
	}
}
