package discord

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// mockWSServer simulates a Discord Gateway websocket endpoint over net.Pipe.
type mockWSServer struct {
	clientConn net.Conn
	serverConn net.Conn
	clientWS   *wsConn
	serverWS   *wsConn
}

func newMockWSServer(t *testing.T) *mockWSServer {
	t.Helper()
	c, s := net.Pipe()
	return &mockWSServer{
		clientConn: c,
		serverConn: s,
		clientWS:   &wsConn{conn: c, r: bufio.NewReader(c)},
		serverWS:   &wsConn{conn: s, r: bufio.NewReader(s)},
	}
}

func (m *mockWSServer) Close() {
	m.clientConn.Close()
	m.serverConn.Close()
}

func (m *mockWSServer) SendServerPayload(t *testing.T, op int, data any, eventType string) {
	t.Helper()
	d, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("SendServerPayload marshal data: %v", err)
	}
	payload := GatewayPayload{
		Op:   op,
		Data: d,
		Type: eventType,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("SendServerPayload marshal payload: %v", err)
	}

	// Server-to-client frames don't need masking according to standard RFC 6455
	// but the ws.go client expects standard parsing. We write using raw frame manually
	// for mock server side to match standard.

	payloadBytes := raw
	header := []byte{0x80 | wsOpText} // FIN=1, opcode=Text

	length := len(payloadBytes)
	switch {
	case length <= 125:
		header = append(header, byte(length))
	case length <= 65535:
		header = append(header, 126, byte(length>>8), byte(length))
	default:
		t.Fatalf("SendServerPayload too large")
	}

	_, err = m.serverConn.Write(append(header, payloadBytes...))
	if err != nil && err.Error() != "io: read/write on closed pipe" && err.Error() != "EOF" {
		t.Logf("SendServerPayload write err (often expected if client closed): %v", err)
	}
}

func (m *mockWSServer) ExpectClientIdentify(t *testing.T) {
	t.Helper()
	raw, err := m.serverWS.ReadText()
	if err != nil {
		t.Fatalf("ExpectClientIdentify ReadText: %v", err)
	}
	var payload GatewayPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("ExpectClientIdentify unmarshal payload: %v", err)
	}
	if payload.Op != OpcodeIdentify {
		t.Fatalf("Expected OpcodeIdentify (2), got %d", payload.Op)
	}
}

func TestGateway_Run_Reconnect(t *testing.T) {
	// Temporarily reduce backoff to avoid test delays
	origBase := gatewayBackoffBase
	origMax := gatewayBackoffMax
	gatewayBackoffBase = 10 * time.Millisecond
	gatewayBackoffMax = 50 * time.Millisecond
	defer func() {
		gatewayBackoffBase = origBase
		gatewayBackoffMax = origMax
	}()

	var connectCalls int32
	mockDialer := func(string) (*wsConn, error) {
		atomic.AddInt32(&connectCalls, 1)
		return nil, fmt.Errorf("mock dial error") //nolint:perfsprint // Keep as fmt.Errorf to match style, error is only used for trace
	}

	g := NewGateway("token", DefaultIntents, nil, newTestLogger())
	g.dial = mockDialer

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		g.Run(ctx, "wss://mock")
		close(done)
	}()

	// Wait enough time for a few retries to occur
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	calls := atomic.LoadInt32(&connectCalls)
	if calls < 2 {
		t.Errorf("Expected at least 2 connect attempts (retries), got %d", calls)
	}
}

func TestGateway_Connect_Success(t *testing.T) {
	mockServer := newMockWSServer(t)
	defer mockServer.Close()

	g := NewGateway("token", DefaultIntents, nil, newTestLogger())
	g.dial = func(string) (*wsConn, error) {
		return mockServer.clientWS, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- g.connect(ctx, "wss://mock")
	}()

	// 1. Send Hello from server
	mockServer.SendServerPayload(t, OpcodeHello, GatewayHello{HeartbeatInterval: 1000}, "")

	// 2. Expect Identify from client
	mockServer.ExpectClientIdentify(t)

	// 3. Client will now enter event loop & heartbeat loop.
	// We send a normal event to see if it processes it without dying.
	mockServer.SendServerPayload(t, OpcodeDispatch, ReadyEvent{V: 10, User: &User{ID: "bot123", Username: "TestBot"}}, "READY")

	// Ensure we can cancel and it cleans up nicely
	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-errCh
	if err != nil && err.Error() != "context canceled" && err.Error() != "io: read/write on closed pipe" {
		t.Fatalf("Expected clean exit or cancellation, got: %v", err)
	}
}

func TestGateway_HandlePayload_Opcodes(t *testing.T) {
	g := NewGateway("token", DefaultIntents, nil, newTestLogger())
	ctx := context.Background()

	// Reconnect
	pReconnect := &GatewayPayload{Op: OpcodeReconnect}
	err := g.handlePayload(ctx, pReconnect)
	if err == nil || err.Error() != "server requested reconnect (op=7)" {
		t.Errorf("Expected reconnect error, got %v", err)
	}

	// Invalid Session
	pInvalid := &GatewayPayload{Op: OpcodeInvalidSession}
	err = g.handlePayload(ctx, pInvalid)
	if err == nil || err.Error() != "invalid session (op=9)" {
		t.Errorf("Expected invalid session error, got %v", err)
	}

	// HeartbeatACK - Should not error
	pAck := &GatewayPayload{Op: OpcodeHeartbeatACK}
	if err := g.handlePayload(ctx, pAck); err != nil {
		t.Errorf("Expected nil for HeartbeatACK, got %v", err)
	}
}

func TestGateway_HandleDispatch(t *testing.T) {
	msgCh := make(chan *Message, 1)
	handler := func(ctx context.Context, eventType string, msg *Message) {
		msgCh <- msg
	}

	g := NewGateway("token", DefaultIntents, handler, newTestLogger())
	ctx := context.Background()

	t.Run("MESSAGE_CREATE from user", func(t *testing.T) {
		// Clear channel
		select {
		case <-msgCh:
		default:
		}

		msg := Message{ID: "123", Content: "hello", Author: &User{Bot: false}}
		d, _ := json.Marshal(msg)
		p := &GatewayPayload{Op: OpcodeDispatch, Type: "MESSAGE_CREATE", Data: d}

		err := g.handleDispatch(ctx, p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		select {
		case handledMessage := <-msgCh:
			if handledMessage.ID != "123" {
				t.Errorf("Expected message ID 123, got %s", handledMessage.ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Expected message to be handled")
		}
	})

	t.Run("MESSAGE_CREATE from bot", func(t *testing.T) {
		// Clear channel
		select {
		case <-msgCh:
		default:
		}

		msg := Message{ID: "456", Content: "bot spam", Author: &User{Bot: true}}
		d, _ := json.Marshal(msg)
		p := &GatewayPayload{Op: OpcodeDispatch, Type: "MESSAGE_CREATE", Data: d}

		err := g.handleDispatch(ctx, p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		select {
		case <-msgCh:
			t.Error("Expected bot message to be ignored")
		case <-time.After(50 * time.Millisecond):
			// Success, no message received
		}
	})
}
