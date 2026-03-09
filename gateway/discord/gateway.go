package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// EventHandler is called for each Discord dispatch event (op=0) that carries
// a *Message payload (e.g. MESSAGE_CREATE). The eventType argument is the
// raw Discord event name ("MESSAGE_CREATE", "READY", …).
type EventHandler func(ctx context.Context, eventType string, msg *Message)

// Gateway manages the Discord WebSocket connection, the heartbeat ticker, and
// the full session lifecycle (Hello → Identify → event loop).
//
// It reconnects automatically on transient failures using exponential backoff.
// A permanent failure (e.g. invalid token) is surfaced only after the first
// attempt; subsequent attempts continue backing off until ctx is cancelled.
type Gateway struct {
	token   string
	intents int
	handler EventHandler
	log     *slog.Logger
}

// NewGateway constructs a Gateway for the given bot token, intent bitfield,
// and event handler. handler may be nil (events are silently discarded).
func NewGateway(token string, intents int, handler EventHandler, log *slog.Logger) *Gateway {
	return &Gateway{
		token:   token,
		intents: intents,
		handler: handler,
		log:     log,
	}
}

// Run connects to the Discord Gateway at gatewayURL and processes events until
// ctx is cancelled. It reconnects with exponential backoff on failures.
//
// gatewayURL should be a wss:// URL as returned by Client.GetGatewayURL; the
// required query string (?v=10&encoding=json) is appended internally.
func (g *Gateway) Run(ctx context.Context, gatewayURL string) {
	const (
		backoffBase = 2 * time.Second
		backoffMax  = 60 * time.Second
	)
	backoff := backoffBase

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wsURL := gatewayURL + "?v=10&encoding=json"
		err := g.connect(ctx, wsURL)
		if err == nil || ctx.Err() != nil {
			return
		}

		g.log.Error("discord_gateway_disconnected",
			slog.String("error", err.Error()),
			slog.Duration("retry_in", backoff),
		)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		if backoff < backoffMax {
			backoff *= 2
		}
	}
}

// connect establishes one complete Discord Gateway session.
// It performs: dial → Hello → Identify → event loop + heartbeat.
// Returns nil on clean ctx cancellation, non-nil on session failure.
func (g *Gateway) connect(ctx context.Context, wsURL string) error {
	ws, err := dialWS(wsURL)
	if err != nil {
		return fmt.Errorf("discord: gateway dial: %w", err)
	}
	defer ws.Close()

	g.log.Info("discord_gateway_connected")

	// Step 1 — receive Hello (op=10)
	helloRaw, err := ws.ReadText()
	if err != nil {
		return fmt.Errorf("discord: gateway hello recv: %w", err)
	}
	var helloEnvelope GatewayPayload
	if err := json.Unmarshal([]byte(helloRaw), &helloEnvelope); err != nil {
		return fmt.Errorf("discord: decode hello envelope: %w", err)
	}
	if helloEnvelope.Op != OpcodeHello {
		return fmt.Errorf("discord: expected op=10 (Hello), got op=%d", helloEnvelope.Op)
	}
	var hello GatewayHello
	if err := json.Unmarshal(helloEnvelope.Data, &hello); err != nil {
		return fmt.Errorf("discord: decode hello data: %w", err)
	}

	// Step 2 — send Identify (op=2)
	if err := g.sendIdentify(ws); err != nil {
		return err
	}

	heartbeatInterval := time.Duration(hello.HeartbeatInterval) * time.Millisecond

	// Step 3 — run heartbeat in parallel with the receive loop.
	// Use an inner cancellable context so the heartbeat goroutine stops
	// immediately when the receive loop exits (error or ctx cancellation).
	connCtx, cancelConn := context.WithCancel(ctx)
	defer cancelConn()

	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		g.heartbeat(connCtx, ws, heartbeatInterval)
	}()

	// Step 4 — receive events until ctx cancelled or connection drops.
	recvErr := g.receiveLoop(ctx, ws)
	cancelConn() // stop heartbeat before waiting for it
	<-heartbeatDone

	return recvErr
}

// sendIdentify transmits the op=2 Identify payload containing the bot token
// and requested intent flags.
func (g *Gateway) sendIdentify(ws *wsConn) error {
	identifyData, err := json.Marshal(GatewayIdentify{
		Token:   g.token,
		Intents: g.intents,
		Properties: map[string]string{
			"os":      "linux",
			"browser": "aethercore",
			"device":  "aethercore",
		},
	})
	if err != nil {
		return fmt.Errorf("discord: marshal identify data: %w", err)
	}

	envelope := GatewayPayload{Op: OpcodeIdentify, Data: identifyData}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("discord: marshal identify envelope: %w", err)
	}
	return ws.WriteText(string(raw))
}

// heartbeat sends op=1 Heartbeat frames on the given interval until ctx is
// cancelled or a send fails (which signals the connection is broken).
func (g *Gateway) heartbeat(ctx context.Context, ws *wsConn, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := ws.WriteText(`{"op":1,"d":null}`); err != nil {
				g.log.Error("discord_heartbeat_failed", slog.String("error", err.Error()))
				return
			}
			g.log.Debug("discord_heartbeat_sent")
		}
	}
}

// receiveLoop reads and dispatches incoming Gateway payloads until ctx is
// cancelled or the connection returns a fatal error.
func (g *Gateway) receiveLoop(ctx context.Context, ws *wsConn) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Give the Gateway a generous deadline; HeartbeatACK arrives within ~5 s.
		_ = ws.SetDeadline(time.Now().Add(90 * time.Second))

		text, err := ws.ReadText()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err() //nolint:wrapcheck // propagate cancellation directly
			}
			return fmt.Errorf("discord: ws recv: %w", err)
		}

		var payload GatewayPayload
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			g.log.Error("discord_payload_decode_failed", slog.String("error", err.Error()))
			continue
		}

		if err := g.handlePayload(ctx, &payload); err != nil {
			g.log.Error("discord_handle_payload_failed",
				slog.String("event", payload.Type),
				slog.String("error", err.Error()),
			)
		}
	}
}

// handlePayload routes a single decoded Gateway payload to the right handler.
func (g *Gateway) handlePayload(ctx context.Context, p *GatewayPayload) error {
	switch p.Op {
	case OpcodeDispatch:
		return g.handleDispatch(ctx, p)
	case OpcodeHeartbeatACK:
		g.log.Debug("discord_heartbeat_ack")
	case OpcodeReconnect:
		return errors.New("server requested reconnect (op=7)")
	case OpcodeInvalidSession:
		return errors.New("invalid session (op=9)")
	}
	return nil
}

// handleDispatch processes op=0 (Dispatch) events.
// Only READY and MESSAGE_CREATE are actively handled; all others are ignored.
func (g *Gateway) handleDispatch(ctx context.Context, p *GatewayPayload) error {
	switch p.Type {
	case "READY":
		var ready ReadyEvent
		if err := json.Unmarshal(p.Data, &ready); err != nil {
			return fmt.Errorf("decode READY: %w", err)
		}
		if ready.User != nil {
			g.log.Info("discord_bot_ready",
				slog.String("username", ready.User.Username),
				slog.String("id", ready.User.ID),
			)
		}

	case "MESSAGE_CREATE":
		var msg Message
		if err := json.Unmarshal(p.Data, &msg); err != nil {
			return fmt.Errorf("decode MESSAGE_CREATE: %w", err)
		}
		// Ignore messages sent by bots (including ourselves) to prevent loops.
		if msg.Author != nil && msg.Author.Bot {
			return nil
		}
		if g.handler != nil {
			go g.handler(ctx, p.Type, &msg)
		}
	}
	return nil
}
