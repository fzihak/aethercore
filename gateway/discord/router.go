package discord

import (
	"context"
	"strings"
)

// CommandHandler is called when a Discord message matches a registered command.
// channelID is the Discord channel snowflake ID; args is the text after the command name.
type CommandHandler func(ctx context.Context, channelID string, args string)

// Router dispatches Discord Message events to registered CommandHandlers based
// on a configurable command prefix (e.g. "!").
//
// Non-command messages and unrecognised commands are forwarded to the Fallback
// handler when one is set.
type Router struct {
	prefix   string
	commands map[string]CommandHandler
	fallback CommandHandler
}

// NewRouter creates a Router that recognises commands starting with prefix.
// prefix must be non-empty (e.g. "!" or "/").
func NewRouter(prefix string) *Router {
	return &Router{
		prefix:   prefix,
		commands: make(map[string]CommandHandler),
	}
}

// Register associates a command name (without prefix) with h.
// Names are normalised to lowercase so registration is case-insensitive.
func (r *Router) Register(command string, h CommandHandler) {
	r.commands[strings.ToLower(command)] = h
}

// SetFallback sets the handler called when a message is not a recognised command.
func (r *Router) SetFallback(h CommandHandler) { r.fallback = h }

// Handle satisfies the EventHandler signature and routes msg to the appropriate
// CommandHandler. eventType is accepted but unused (always "MESSAGE_CREATE").
// A nil or empty message is silently ignored.
func (r *Router) Handle(ctx context.Context, _ string, msg *Message) {
	if msg == nil || strings.TrimSpace(msg.Content) == "" {
		return
	}
	text := strings.TrimSpace(msg.Content)
	channelID := msg.ChannelID

	cmd, args := r.parseCommand(text)
	if cmd == "" {
		if r.fallback != nil {
			r.fallback(ctx, channelID, text)
		}
		return
	}

	handler, ok := r.commands[cmd]
	if !ok {
		if r.fallback != nil {
			r.fallback(ctx, channelID, text)
		}
		return
	}
	handler(ctx, channelID, args)
}

// parseCommand splits text into a (command, args) pair.
// Returns an empty cmd when the text does not start with r.prefix.
func (r *Router) parseCommand(text string) (cmd, args string) {
	if !strings.HasPrefix(text, r.prefix) {
		return "", text
	}
	rest := strings.TrimPrefix(text, r.prefix)
	if idx := strings.IndexByte(rest, ' '); idx >= 0 {
		return strings.ToLower(rest[:idx]), strings.TrimSpace(rest[idx+1:])
	}
	return strings.ToLower(rest), ""
}
