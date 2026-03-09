package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fzihak/aethercore/sdk"
)

// ---- echoModule ------------------------------------------------------------

// echoModule is a minimal sdk.Module that echoes its task input.
// Used across adapter tests to simulate a loaded module.
type echoModule struct{}

func (e *echoModule) Manifest() sdk.ModuleManifest {
	return sdk.ModuleManifest{
		Name:        "echo",
		Description: "Echoes input",
		Version:     "0.1.0",
		Author:      "test",
	}
}
func (e *echoModule) OnStart(_ context.Context, _ *sdk.ModuleContext) error { return nil }
func (e *echoModule) OnStop(_ context.Context) error                         { return nil }
func (e *echoModule) HandleTask(_ context.Context, t *sdk.ModuleTask) (*sdk.ModuleResult, error) {
	return &sdk.ModuleResult{TaskID: t.ID, Output: "echo:" + t.Input}, nil
}

// ---- test helpers ----------------------------------------------------------

// captureSendMessage creates a test server that captures the text of the most
// recent sendMessage POST body in *dest.
func captureSendMessage(t *testing.T, dest *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CreateMessageRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		*dest = req.Content
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Message{ID: "1", ChannelID: "123", Content: *dest}) //nolint:errcheck
	}))
}

// newTestAdapter constructs an Adapter backed by an httptest.Server.
func newTestAdapter(srv *httptest.Server, registry *sdk.ModuleRegistry) *Adapter {
	client := newTestClient("token", srv.URL)
	return NewAdapter(client, registry, newTestLogger())
}

// ---- Adapter.HandleRun tests -----------------------------------------------

func TestHandleRun_emptyGoal_repliesUsage(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	adapter := newTestAdapter(srv, sdk.NewModuleRegistry())
	adapter.HandleRun(context.Background(), "123", "")

	if !strings.Contains(sentText, "Usage") {
		t.Errorf("expected 'Usage' hint in reply, got: %q", sentText)
	}
}

func TestHandleRun_noModules_repliesWarning(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	adapter := newTestAdapter(srv, sdk.NewModuleRegistry())
	adapter.HandleRun(context.Background(), "123", "do something")

	if !strings.Contains(sentText, "No modules") {
		t.Errorf("expected 'No modules' warning, got: %q", sentText)
	}
}

func TestHandleRun_withModule_includesOutput(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	registry := sdk.NewModuleRegistry()
	_ = sdk.StartModule(context.Background(), registry, &echoModule{}, sdk.NewModuleContext("echo"))

	adapter := newTestAdapter(srv, registry)
	adapter.HandleRun(context.Background(), "123", "hello")

	if !strings.Contains(sentText, "echo:hello") {
		t.Errorf("expected echo output in reply, got: %q", sentText)
	}
}

func TestHandleRun_taskMetadata_containsDiscordSource(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	var gotMetadata map[string]string
	spy := &spyModule{meta: &gotMetadata}
	registry := sdk.NewModuleRegistry()
	_ = sdk.StartModule(context.Background(), registry, spy, sdk.NewModuleContext("spy"))

	adapter := newTestAdapter(srv, registry)
	adapter.HandleRun(context.Background(), "chan99", "test-goal")

	if gotMetadata["source"] != "discord" {
		t.Errorf("want source=discord, got %q", gotMetadata["source"])
	}
	if gotMetadata["channel_id"] != "chan99" {
		t.Errorf("want channel_id=chan99, got %q", gotMetadata["channel_id"])
	}
}

// ---- Adapter.HandleHelp tests ----------------------------------------------

func TestHandleHelp_containsAllCommands(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	adapter := newTestAdapter(srv, sdk.NewModuleRegistry())
	adapter.HandleHelp(context.Background(), "123", "")

	for _, kw := range []string{"!run", "!modules", "!help", "AetherCore"} {
		if !strings.Contains(sentText, kw) {
			t.Errorf("expected %q in help reply, got: %q", kw, sentText)
		}
	}
}

func TestHandleHelp_withModule_listsModule(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	registry := sdk.NewModuleRegistry()
	_ = sdk.StartModule(context.Background(), registry, &echoModule{}, sdk.NewModuleContext("echo"))

	adapter := newTestAdapter(srv, registry)
	adapter.HandleHelp(context.Background(), "123", "")

	if !strings.Contains(sentText, "echo") {
		t.Errorf("expected module name 'echo' in help reply, got: %q", sentText)
	}
}

// ---- Adapter.HandleModules tests -------------------------------------------

func TestHandleModules_noModules_repliesWarning(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	adapter := newTestAdapter(srv, sdk.NewModuleRegistry())
	adapter.HandleModules(context.Background(), "123", "")

	if !strings.Contains(sentText, "No modules") {
		t.Errorf("expected 'No modules' in reply, got: %q", sentText)
	}
}

func TestHandleModules_withModule_showsManifest(t *testing.T) {
	var sentText string
	srv := captureSendMessage(t, &sentText)
	defer srv.Close()

	registry := sdk.NewModuleRegistry()
	_ = sdk.StartModule(context.Background(), registry, &echoModule{}, sdk.NewModuleContext("echo"))

	adapter := newTestAdapter(srv, registry)
	adapter.HandleModules(context.Background(), "123", "")

	if !strings.Contains(sentText, "echo") {
		t.Errorf("expected module name 'echo' in modules reply, got: %q", sentText)
	}
}

// ---- spyModule -------------------------------------------------------------

// spyModule captures the Metadata of each task it handles.
type spyModule struct{ meta *map[string]string }

func (s *spyModule) Manifest() sdk.ModuleManifest {
	return sdk.ModuleManifest{Name: "spy", Description: "Captures task metadata", Version: "0.1.0", Author: "test"}
}
func (s *spyModule) OnStart(_ context.Context, _ *sdk.ModuleContext) error { return nil }
func (s *spyModule) OnStop(_ context.Context) error                         { return nil }
func (s *spyModule) HandleTask(_ context.Context, t *sdk.ModuleTask) (*sdk.ModuleResult, error) {
	*s.meta = t.Metadata
	return &sdk.ModuleResult{TaskID: t.ID, Output: "ok"}, nil
}
