package telegram

import (
	"context"
	"testing"
)

// ---- parseCommand ----------------------------------------------------------

func TestParseCommand_basicCommand(t *testing.T) {
	cmd, args := parseCommand("/run hello world")
	if cmd != "run" {
		t.Errorf("want cmd=run, got %q", cmd)
	}
	if args != "hello world" {
		t.Errorf("want args='hello world', got %q", args)
	}
}

func TestParseCommand_commandWithBotName(t *testing.T) {
	cmd, args := parseCommand("/help@MyBot")
	if cmd != "help" {
		t.Errorf("want cmd=help, got %q", cmd)
	}
	if args != "" {
		t.Errorf("want empty args, got %q", args)
	}
}

func TestParseCommand_noArgs(t *testing.T) {
	cmd, args := parseCommand("/start")
	if cmd != "start" {
		t.Errorf("want cmd=start, got %q", cmd)
	}
	if args != "" {
		t.Errorf("want empty args, got %q", args)
	}
}

func TestParseCommand_plainText(t *testing.T) {
	cmd, args := parseCommand("just plain text")
	if cmd != "" {
		t.Errorf("want empty cmd, got %q", cmd)
	}
	if args != "just plain text" {
		t.Errorf("want args='just plain text', got %q", args)
	}
}

func TestParseCommand_upperCaseNormalised(t *testing.T) {
	cmd, _ := parseCommand("/RUN goal")
	if cmd != "run" {
		t.Errorf("want cmd=run (lower-cased), got %q", cmd)
	}
}

// ---- Router.Handle ---------------------------------------------------------

func TestRouter_registeredCommand(t *testing.T) {
	router := NewRouter()
	var gotChatID int64
	var gotArgs string
	router.Register("run", func(_ context.Context, chatID int64, args string) {
		gotChatID = chatID
		gotArgs = args
	})

	upd := buildUpdate(42, "/run hello world")
	router.Handle(context.Background(), upd)

	if gotChatID != 42 {
		t.Errorf("want chatID=42, got %d", gotChatID)
	}
	if gotArgs != "hello world" {
		t.Errorf("want args='hello world', got %q", gotArgs)
	}
}

func TestRouter_unknownCommand_fallback(t *testing.T) {
	router := NewRouter()
	var fallbackCalled bool
	router.SetFallback(func(_ context.Context, _ int64, _ string) {
		fallbackCalled = true
	})

	upd := buildUpdate(1, "/unknown arg")
	router.Handle(context.Background(), upd)

	if !fallbackCalled {
		t.Error("expected fallback to be called for unknown command")
	}
}

func TestRouter_plainText_callsFallback(t *testing.T) {
	router := NewRouter()
	var fallbackText string
	router.SetFallback(func(_ context.Context, _ int64, text string) {
		fallbackText = text
	})

	upd := buildUpdate(1, "plain text message")
	router.Handle(context.Background(), upd)

	if fallbackText != "plain text message" {
		t.Errorf("want fallback text='plain text message', got %q", fallbackText)
	}
}

func TestRouter_nilMessage_ignored(t *testing.T) {
	router := NewRouter()
	var called bool
	router.SetFallback(func(_ context.Context, _ int64, _ string) { called = true })

	router.Handle(context.Background(), Update{UpdateID: 1, Message: nil})

	if called {
		t.Error("fallback must not be called for nil message")
	}
}

func TestRouter_emptyText_ignored(t *testing.T) {
	router := NewRouter()
	var called bool
	router.SetFallback(func(_ context.Context, _ int64, _ string) { called = true })

	upd := buildUpdate(1, "")
	router.Handle(context.Background(), upd)

	if called {
		t.Error("fallback must not be called for empty text")
	}
}

// buildUpdate is a test helper that constructs an Update with a private message.
func buildUpdate(chatID int64, text string) Update {
	return Update{
		UpdateID: 1,
		Message: &Message{
			MessageID: 1,
			Chat:      Chat{ID: chatID, Type: "private"},
			Date:      0,
			Text:      text,
		},
	}
}
