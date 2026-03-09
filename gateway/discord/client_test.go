package discord

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// makeServer creates an httptest.Server that serves one canned JSON body for
// any incoming request.
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

// ---- GetGatewayURL ---------------------------------------------------------

func TestGetGatewayURL_success(t *testing.T) {
	srv := makeServer(t, map[string]any{"url": "wss://gateway.discord.gg", "shards": 1})
	defer srv.Close()

	c := newTestClient("token", srv.URL)
	got, err := c.GetGatewayURL(context.Background())
	if err != nil {
		t.Fatalf("GetGatewayURL: %v", err)
	}
	if got != "wss://gateway.discord.gg" {
		t.Errorf("want wss://gateway.discord.gg, got %q", got)
	}
}

func TestGetGatewayURL_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(APIError{Code: 0, Message: "401: Unauthorized"})
	}))
	defer srv.Close()

	c := newTestClient("bad-token", srv.URL)
	_, err := c.GetGatewayURL(context.Background())
	if err == nil {
		t.Fatal("expected error from GetGatewayURL with 401, got nil")
	}
}

// ---- SendMessage -----------------------------------------------------------

func TestSendMessage_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Message{
			ID:        "999",
			ChannelID: "456",
			Content:   "hello",
		})
	}))
	defer srv.Close()

	c := newTestClient("token", srv.URL)
	msg, err := c.SendMessage(context.Background(), "456", "hello")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if msg.ChannelID != "456" {
		t.Errorf("want channelID=456, got %q", msg.ChannelID)
	}
}

func TestSendMessage_apiError_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(APIError{Code: 50013, Message: "Missing Permissions"})
	}))
	defer srv.Close()

	c := newTestClient("token", srv.URL)
	_, err := c.SendMessage(context.Background(), "1", "hi")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 50013 {
		t.Errorf("want code 50013, got %d", apiErr.Code)
	}
}

// ---- APIError --------------------------------------------------------------

func TestAPIError_message(t *testing.T) {
	e := &APIError{Code: 50013, Message: "Missing Permissions"}
	got := e.Error()
	want := "discord: API error 50013: Missing Permissions"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestClient_do_genericHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	c := newTestClient("token", srv.URL)
	_, err := c.SendMessage(context.Background(), "1", "hi")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
