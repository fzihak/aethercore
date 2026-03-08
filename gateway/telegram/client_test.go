package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// makeServer creates an httptest.Server that serves one canned JSON body for
// any request, then returns the server URL.
func makeServer(t *testing.T, body any) *httptest.Server {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("makeServer: marshal: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
}

// okResponse wraps any result in a Telegram ok=true API envelope.
func okResponse(result any) map[string]any {
	return map[string]any{"ok": true, "result": result}
}

// errResponse returns a Telegram ok=false error envelope.
func errResponse(code int, desc string) map[string]any {
	return map[string]any{"ok": false, "error_code": code, "description": desc}
}

// ---- GetMe -----------------------------------------------------------------

func TestGetMe_success(t *testing.T) {
	srv := makeServer(t, okResponse(map[string]any{
		"id": 42, "is_bot": true, "first_name": "AetherBot", "username": "aetherbot",
	}))
	defer srv.Close()

	c := newTestClient("token", srv.URL+"/bot")
	me, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if me.ID != 42 {
		t.Errorf("want ID=42, got %d", me.ID)
	}
	if me.Username != "aetherbot" {
		t.Errorf("want username=aetherbot, got %q", me.Username)
	}
}

func TestGetMe_apiError(t *testing.T) {
	srv := makeServer(t, errResponse(401, "Unauthorized"))
	defer srv.Close()

	c := newTestClient("bad", srv.URL+"/bot")
	_, err := c.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error from GetMe with 401, got nil")
	}
	var apiErr *APIError
	if !isAPIError(err, &apiErr) {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 401 {
		t.Errorf("want code 401, got %d", apiErr.Code)
	}
}

// ---- SendMessage -----------------------------------------------------------

func TestSendMessage_success(t *testing.T) {
	srv := makeServer(t, okResponse(map[string]any{
		"message_id": 1, "chat": map[string]any{"id": 99, "type": "private"}, "date": 0, "text": "hello",
	}))
	defer srv.Close()

	c := newTestClient("token", srv.URL+"/bot")
	msg, err := c.SendMessage(context.Background(), 99, "hello")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if msg.Chat.ID != 99 {
		t.Errorf("want chat_id=99, got %d", msg.Chat.ID)
	}
}

// ---- GetUpdates ------------------------------------------------------------

func TestGetUpdates_empty(t *testing.T) {
	srv := makeServer(t, okResponse([]Update{}))
	defer srv.Close()

	c := newTestClient("token", srv.URL+"/bot")
	updates, err := c.GetUpdates(context.Background(), 0, 1)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("want 0 updates, got %d", len(updates))
	}
}

func TestGetUpdates_withMessages(t *testing.T) {
	payload := []map[string]any{
		{
			"update_id": 100,
			"message": map[string]any{
				"message_id": 1, "date": 0, "text": "/help",
				"chat": map[string]any{"id": 5, "type": "private"},
			},
		},
	}
	srv := makeServer(t, okResponse(payload))
	defer srv.Close()

	c := newTestClient("token", srv.URL+"/bot")
	updates, err := c.GetUpdates(context.Background(), 0, 1)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("want 1 update, got %d", len(updates))
	}
	if updates[0].UpdateID != 100 {
		t.Errorf("want UpdateID=100, got %d", updates[0].UpdateID)
	}
	if updates[0].Message.Text != "/help" {
		t.Errorf("want text=/help, got %q", updates[0].Message.Text)
	}
}

// ---- APIError --------------------------------------------------------------

func TestAPIError_message(t *testing.T) {
	e := &APIError{Code: 400, Description: "Bad Request"}
	got := e.Error()
	want := "telegram: API error 400: Bad Request"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

// isAPIError is a type-assertion helper (errors.As requires a pointer-to-interface).
func isAPIError(err error, target **APIError) bool {
	if err == nil {
		return false
	}
	if ae, ok := err.(*APIError); ok {
		*target = ae
		return true
	}
	return false
}
