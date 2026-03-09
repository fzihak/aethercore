// Package discord provides a native Discord gateway adapter for AetherCore.
// The WebSocket implementation is written from scratch using only the Go
// standard library so that the gateway/discord package introduces zero new
// module dependencies (RFC 6455, TLS via crypto/tls).
package discord

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 — RFC 6455 mandates SHA-1 for WebSocket handshake
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// wsGUID is the magic GUID defined in RFC 6455 §1.3 for the handshake.
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// WebSocket frame opcodes (RFC 6455 §5.2).
const (
	wsOpContinuation byte = 0x0
	wsOpText         byte = 0x1
	wsOpBinary       byte = 0x2
	wsOpClose        byte = 0x8
	wsOpPing         byte = 0x9
	wsOpPong         byte = 0xA
)

// wsConn is a minimal WebSocket client connection over a TLS transport.
// Only one goroutine must call ReadText and only one must call WriteText
// concurrently — the two directions are fully independent.
type wsConn struct {
	conn net.Conn
	r    *bufio.Reader
}

// dialWS opens a TLS WebSocket connection to the given wss:// URL and
// performs the HTTP/1.1 Upgrade handshake (RFC 6455 §4.1).
// Only wss:// (TLS) is supported; Discord never terminates plain-text ws://.
func dialWS(rawURL string) (*wsConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("discord: ws parse url: %w", err)
	}
	if u.Scheme != "wss" {
		return nil, fmt.Errorf("discord: ws only wss:// supported, got %q", u.Scheme)
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 15 * time.Second},
		"tcp", host,
		&tls.Config{
			ServerName: u.Hostname(),
			MinVersion: tls.VersionTLS12,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("discord: ws tls dial %s: %w", host, err)
	}

	wsKey, wsAccept, err := generateWSKey()
	if err != nil {
		_ = conn.Close() // best-effort cleanup
		return nil, fmt.Errorf("discord: ws generate key: %w", err)
	}

	// RFC 6455 §4.1 — opening handshake sent as an HTTP/1.1 request.
	path := u.RequestURI()
	upgradeReq := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + u.Hostname() + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + wsKey + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"

	if _, writeErr := io.WriteString(conn, upgradeReq); writeErr != nil {
		_ = conn.Close() // best-effort cleanup
		return nil, fmt.Errorf("discord: ws upgrade request: %w", writeErr)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		_ = conn.Close() // best-effort cleanup
		return nil, fmt.Errorf("discord: ws read upgrade response: %w", err)
	}
	_ = resp.Body.Close() // upgrade response body is empty

	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close() // best-effort cleanup
		return nil, fmt.Errorf("discord: ws upgrade failed: HTTP %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Sec-WebSocket-Accept"); got != wsAccept {
		_ = conn.Close() // best-effort cleanup
		return nil, fmt.Errorf("discord: ws accept mismatch: got %q want %q", got, wsAccept)
	}

	return &wsConn{conn: conn, r: br}, nil
}

// generateWSKey produces a random Sec-WebSocket-Key and its expected Accept
// value as defined in RFC 6455 §4.1.
func generateWSKey() (key, accept string, _ error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	key = base64.StdEncoding.EncodeToString(b[:])
	h := sha1.Sum([]byte(key + wsGUID)) // #nosec G401 — RFC 6455 §4.1 mandates SHA-1
	accept = base64.StdEncoding.EncodeToString(h[:])
	return key, accept, nil
}

// ReadText reads one complete text message from the server, assembling
// fragmented frames transparently and responding to Ping/Close frames.
// It returns io.EOF on a clean connection close.
func (c *wsConn) ReadText() (string, error) {
	var payload []byte

	for {
		fin, opcode, data, err := c.readFrame()
		if err != nil {
			return "", err
		}

		switch opcode {
		case wsOpClose:
			_ = c.sendClose() // best-effort echo
			return "", io.EOF

		case wsOpPing:
			if err := c.writeFrame(wsOpPong, data); err != nil {
				return "", fmt.Errorf("discord: ws pong: %w", err)
			}

		case wsOpPong:
			// unsolicited pong — discard

		case wsOpText, wsOpBinary, wsOpContinuation:
			payload = append(payload, data...)
			if fin {
				return string(payload), nil
			}

		default:
			// unknown opcode — ignore
		}
	}
}

// WriteText sends payload as a single masked text frame (RFC 6455 §5.1:
// client-to-server frames MUST be masked).
func (c *wsConn) WriteText(text string) error {
	return c.writeFrame(wsOpText, []byte(text))
}

// Close sends a Close frame and shuts down the underlying connection.
func (c *wsConn) Close() error {
	_ = c.sendClose()
	return c.conn.Close()
}

// SetDeadline sets an absolute read/write deadline on the underlying conn.
func (c *wsConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// sendClose transmits a Close frame (opcode 0x8) with no status code.
func (c *wsConn) sendClose() error {
	return c.writeFrame(wsOpClose, nil)
}

// readFrame parses a single WebSocket frame from the buffered reader.
// Server-to-client frames are never masked (RFC 6455 §5.1), but this
// implementation unmasks frames if the MASK bit is set for robustness.
func (c *wsConn) readFrame() (fin bool, opcode byte, payload []byte, _ error) {
	b0, err := c.r.ReadByte()
	if err != nil {
		return false, 0, nil, fmt.Errorf("discord: ws read header byte 0: %w", err)
	}
	fin = (b0 & 0x80) != 0
	opcode = b0 & 0x0F

	b1, err := c.r.ReadByte()
	if err != nil {
		return false, 0, nil, fmt.Errorf("discord: ws read header byte 1: %w", err)
	}
	masked := (b1 & 0x80) != 0
	length := int64(b1 & 0x7F)

	switch length {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.r, ext[:]); err != nil {
			return false, 0, nil, fmt.Errorf("discord: ws read 16-bit length: %w", err)
		}
		length = int64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.r, ext[:]); err != nil {
			return false, 0, nil, fmt.Errorf("discord: ws read 64-bit length: %w", err)
		}
		rawLen := binary.BigEndian.Uint64(ext[:])
		if rawLen > 1<<63-1 { // guard against uint64 overflow when casting to int64
			return false, 0, nil, fmt.Errorf("discord: ws frame length overflow: %d", rawLen)
		}
		length = int64(rawLen) // #nosec G115 — guarded by overflow check above
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.r, maskKey[:]); err != nil {
			return false, 0, nil, fmt.Errorf("discord: ws read mask key: %w", err)
		}
	}

	if length > 0 {
		payload = make([]byte, length)
		if _, err := io.ReadFull(c.r, payload); err != nil {
			return false, 0, nil, fmt.Errorf("discord: ws read payload: %w", err)
		}
		if masked {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}
	}

	return fin, opcode, payload, nil
}

// writeFrame encodes and sends a single WebSocket frame with a fresh random
// masking key (RFC 6455 §5.3 — client-to-server frames MUST be masked).
func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	var maskKey [4]byte
	if _, err := rand.Read(maskKey[:]); err != nil {
		return fmt.Errorf("discord: ws generate mask key: %w", err)
	}

	length := len(payload)

	// Pre-allocate header: 2 bytes minimum + up to 8 bytes extended length + 4 bytes mask.
	header := make([]byte, 0, 14)
	header = append(header, 0x80|opcode) // FIN=1, RSV=0, opcode

	switch {
	case length <= 125:
		header = append(header, 0x80|byte(length))
	case length <= 65535:
		header = append(header, 0x80|126)
		header = binary.BigEndian.AppendUint16(header, uint16(length))
	default:
		header = append(header, 0x80|127)
		header = binary.BigEndian.AppendUint64(header, uint64(length))
	}

	header = append(header, maskKey[:]...)

	masked := make([]byte, length)
	for i, b := range payload {
		masked[i] = b ^ maskKey[i%4]
	}

	frame := make([]byte, 0, len(header)+len(masked))
	frame = append(frame, header...)
	frame = append(frame, masked...)
	if _, err := c.conn.Write(frame); err != nil {
		return fmt.Errorf("discord: ws write frame: %w", err)
	}
	return nil
}
