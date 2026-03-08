package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fzihak/aethercore/sdk"
)

// ---- Adapter.HandleRun -----------------------------------------------------

func TestHandleRun_noModules_repliesWarning(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	registry := sdk.NewModuleRegistry()
	adapter := newTestAdapter(srv, registry)

	adapter.HandleRun(context.Background(), 1, "do something")

	if !strings.Contains(sentText, "No modules") {
		t.Errorf("expected 'No modules' warning, got: %q", sentText)
	}
}

func TestHandleRun_emptyGoal_repliesUsage(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	registry := sdk.NewModuleRegistry()
	adapter := newTestAdapter(srv, registry)

	adapter.HandleRun(context.Background(), 1, "")

	if !strings.Contains(sentText, "Usage") {
		t.Errorf("expected Usage hint, got: %q", sentText)
	}
}

func TestHandleRun_withModule_includesOutput(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	registry := sdk.NewModuleRegistry()
	mod := &echoModule{}
	mc := sdk.NewModuleContext("echo")
	_ = sdk.StartModule(context.Background(), registry, mod, mc)

	adapter := newTestAdapter(srv, registry)
	adapter.HandleRun(context.Background(), 1, "hello")

	if !strings.Contains(sentText, "echo:hello") {
		t.Errorf("expected echo output in reply, got: %q", sentText)
	}
}

// ---- Adapter.HandleHelp / HandleModules ------------------------------------

func TestHandleHelp_containsCommands(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	adapter := newTestAdapter(srv, sdk.NewModuleRegistry())
	adapter.HandleHelp(context.Background(), 1, "")

	for _, keyword := range []string{"/run", "/modules", "/help", "AetherCore"} {
		if !strings.Contains(sentText, keyword) {
			t.Errorf("expected %q in help text, got: %q", keyword, sentText)
		}
	}
}

func TestHandleModules_noModules(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	adapter := newTestAdapter(srv, sdk.NewModuleRegistry())
	adapter.HandleModules(context.Background(), 1, "")

	if !strings.Contains(sentText, "No modules") {
		t.Errorf("expected 'No modules' message, got: %q", sentText)
	}
}

func TestHandleModules_withModule_showsManifest(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	registry := sdk.NewModuleRegistry()
	mod := &echoModule{}
	mc := sdk.NewModuleContext("echo")
	_ = sdk.StartModule(context.Background(), registry, mod, mc)

	adapter := newTestAdapter(srv, registry)
	adapter.HandleModules(context.Background(), 1, "")

	if !strings.Contains(sentText, "echo") {
		t.Errorf("expected module name 'echo' in reply, got: %q", sentText)
	}
}

// ---- helpers ---------------------------------------------------------------

// captureSendMessage builds a test server that intercepts sendMessage calls
// and stores the text body in *dest.
func captureSendMessage(t *testing.T, dest *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "sendMessage") {
			var req SendMessageRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			*dest = req.Text
		}
		// Always respond with a valid Telegram envelope so the client doesn't error.
		resp := okResponse(map[string]any{
			"message_id": 1,
			"chat":       map[string]any{"id": 1, "type": "private"},
			"date":       0,
			"text":       *dest,
		})
		raw, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
}

// newTestAdapter constructs an Adapter backed by a test HTTP server.
func newTestAdapter(srv *httptest.Server, registry *sdk.ModuleRegistry) *Adapter {
	client := newTestClient("token", srv.URL+"/bot")
	log := newTestLogger()
	return NewAdapter(client, registry, log)
}

// echoModule is a minimal sdk.Module that echoes its task input.
type echoModule struct{}

func (e *echoModule) Manifest() sdk.ModuleManifest {
	return sdk.ModuleManifest{
		Name:             "echo",
		Description:      "Echoes input",
		Version:          "1.0.0",
		Author:           "test",
		MaxTaskRuntimeMs: 1000,
	}
}
func (e *echoModule) OnStart(_ context.Context, _ *sdk.ModuleContext) error { return nil }
func (e *echoModule) OnStop(_ context.Context) error                        { return nil }
func (e *echoModule) HandleTask(_ context.Context, t *sdk.ModuleTask) (*sdk.ModuleResult, error) {
	return &sdk.ModuleResult{TaskID: t.ID, Output: "echo:" + t.Input}, nil
}
