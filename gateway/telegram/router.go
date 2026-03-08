package telegram

import (
	"context"
	"strings"
)

// CommandHandler is called when a registered bot command is received.
// chatID is the Telegram chat to reply to; args is the text after the command.
type CommandHandler func(ctx context.Context, chatID int64, args string)

// Router maps Telegram bot commands (e.g. "/run") to CommandHandlers and
// dispatches incoming updates accordingly.
//
// Non-command messages (plain text, media, etc.) are forwarded to a
// configurable fallback handler.
type Router struct {
	commands map[string]CommandHandler // key: command without leading "/"
	fallback CommandHandler
}

// NewRouter constructs an empty Router.
// Use Register and SetFallback to wire up handlers before passing to a Poller.
func NewRouter() *Router {
	return &Router{commands: make(map[string]CommandHandler)}
}

// Register adds a handler for the given slash-command (without the leading "/").
// Example: router.Register("run", myHandler)  →  handles "/run <args>".
func (r *Router) Register(command string, h CommandHandler) {
	r.commands[strings.ToLower(command)] = h
}

// SetFallback sets the handler for messages that are not bot commands.
// If not set, non-command messages are silently ignored.
func (r *Router) SetFallback(h CommandHandler) {
	r.fallback = h
}

// Handle implements UpdateHandler.  It parses the incoming update's text,
// extracts a leading /command and the remainder, looks up the registered
// handler, and calls it.  Bot-name suffixes (e.g. "/run@MyBot") are stripped.
func (r *Router) Handle(ctx context.Context, upd Update) {
	if upd.Message == nil || upd.Message.Text == "" {
		return
	}

	msg := upd.Message
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	cmd, args := parseCommand(text)
	if cmd == "" {
		// Plain message — invoke fallback if set.
		if r.fallback != nil {
			r.fallback(ctx, chatID, text)
		}
		return
	}

	handler, ok := r.commands[cmd]
	if !ok {
		// Unknown command — invoke fallback if set.
		if r.fallback != nil {
			r.fallback(ctx, chatID, text)
		}
		return
	}

	handler(ctx, chatID, args)
}

// parseCommand splits a Telegram message text into a lower-case command name
// and the remaining argument string.
//
// Examples:
//
//	"/run hello world"   → ("run", "hello world")
//	"/help@MyBot"        → ("help", "")
//	"just text"          → ("", "just text")
func parseCommand(text string) (string, string) {
	if !strings.HasPrefix(text, "/") {
		return "", text
	}

	// Strip leading slash and split on first whitespace.
	rest := text[1:]
	var name, args string
	if idx := strings.IndexByte(rest, ' '); idx >= 0 {
		name = rest[:idx]
		args = strings.TrimSpace(rest[idx+1:])
	} else {
		name = rest
	}

	// Strip bot-name suffix: "/run@MyBot" → "run"
	if idx := strings.IndexByte(name, '@'); idx >= 0 {
		name = name[:idx]
	}

	return strings.ToLower(name), args
}
