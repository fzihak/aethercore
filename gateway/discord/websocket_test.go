package discord

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

// TestGenerateWSKey verifies that generateWSKey produces unique, non-empty values.
func TestGenerateWSKey(t *testing.T) {
	key1, accept1, err := generateWSKey()
	if err != nil {
		t.Fatalf("generateWSKey: %v", err)
	}
	if key1 == "" {
		t.Error("expected non-empty key")
	}
	if accept1 == "" {
		t.Error("expected non-empty accept")
	}

	key2, _, err := generateWSKey()
	if err != nil {
		t.Fatalf("generateWSKey second call: %v", err)
	}
	if key1 == key2 {
		t.Error("expected different keys on each call (entropy failure)")
	}
}

// TestWebSocketFrameRoundTrip verifies masked write + transparent unmask read
// through a synchronous net.Pipe() (no TLS, no network required).
func TestWebSocketFrameRoundTrip(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	want := `{"op":10,"d":{"heartbeat_interval":41250}}`

	errCh := make(chan error, 1)
	go func() {
		cWS := &wsConn{conn: clientConn, r: bufio.NewReader(clientConn)}
		errCh <- cWS.WriteText(want)
	}()

	// Server side reads the masked client frame and unmasks it.
	sWS := &wsConn{conn: serverConn, r: bufio.NewReader(serverConn)}
	got, err := sWS.ReadText()
	if err != nil {
		t.Fatalf("ReadText: %v", err)
	}
	if writeErr := <-errCh; writeErr != nil {
		t.Fatalf("WriteText: %v", writeErr)
	}
	if got != want {
		t.Errorf("round-trip mismatch:\n want %q\n got  %q", want, got)
	}
}

// TestWebSocketLargePayload exercises the 16-bit extended length path (>125 bytes).
func TestWebSocketLargePayload(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// 200-byte payload triggers the 126-prefix extended length encoding.
	want := strings.Repeat("A", 200)

	errCh := make(chan error, 1)
	go func() {
		cWS := &wsConn{conn: clientConn, r: bufio.NewReader(clientConn)}
		errCh <- cWS.WriteText(want)
	}()

	sWS := &wsConn{conn: serverConn, r: bufio.NewReader(serverConn)}
	got, err := sWS.ReadText()
	if err != nil {
		t.Fatalf("ReadText (large): %v", err)
	}
	if writeErr := <-errCh; writeErr != nil {
		t.Fatalf("WriteText (large): %v", writeErr)
	}
	if got != want {
		t.Errorf("large payload length mismatch: want len=%d got len=%d", len(want), len(got))
	}
}

// TestWebSocketCloseFrame verifies that a Close frame from the server causes
// ReadText to return a non-nil error (io.EOF).
func TestWebSocketCloseFrame(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	go func() {
		// Server sends a close frame (unmasked, FIN=1, opcode=0x8, length=0).
		closeFrame := []byte{0x88, 0x00}
		_, _ = serverConn.Write(closeFrame)
		// Close the server side so the client's echo-close write doesn't block.
		serverConn.Close()
	}()

	cWS := &wsConn{conn: clientConn, r: bufio.NewReader(clientConn)}
	_, err := cWS.ReadText()
	if err == nil {
		t.Fatal("expected non-nil error on close frame, got nil")
	}
}

// TestWebSocketPingPong verifies that the client transparently handles a Ping
// by sending a Pong and then continues reading the next text frame.
func TestWebSocketPingPong(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	want := "after-ping"

	go func() {
		// 1. Send a Ping frame (FIN=1, opcode=0x9, unmasked, no payload).
		pingFrame := []byte{0x89, 0x00}
		_, _ = serverConn.Write(pingFrame)

		// 2. Consume the Pong sent back by the client.
		pongBuf := make([]byte, 10)
		_, _ = serverConn.Read(pongBuf)

		// 3. Send a Text frame with the real payload (unmasked, FIN=1).
		payload := []byte(want)
		_, _ = serverConn.Write([]byte{0x81, byte(len(payload))})
		_, _ = serverConn.Write(payload)
	}()

	cWS := &wsConn{conn: clientConn, r: bufio.NewReader(clientConn)}
	got, err := cWS.ReadText()
	if err != nil {
		t.Fatalf("ReadText after ping: %v", err)
	}
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}
