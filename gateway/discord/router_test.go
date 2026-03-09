package discord

import (
	"context"
	"testing"
)

// ---- parseCommand ----------------------------------------------------------

func TestParseCommand_withArgs(t *testing.T) {
	r := NewRouter("!")
	cmd, args := r.parseCommand("!run hello world")
	if cmd != "run" {
		t.Errorf("want cmd=run, got %q", cmd)
	}
	if args != "hello world" {
		t.Errorf("want args='hello world', got %q", args)
	}
}

func TestParseCommand_noArgs(t *testing.T) {
	r := NewRouter("!")
	cmd, args := r.parseCommand("!help")
	if cmd != "help" {
		t.Errorf("want cmd=help, got %q", cmd)
	}
	if args != "" {
		t.Errorf("want empty args, got %q", args)
	}
}

func TestParseCommand_noPrefix_plainText(t *testing.T) {
	r := NewRouter("!")
	cmd, args := r.parseCommand("just plain text")
	if cmd != "" {
		t.Errorf("want empty cmd, got %q", cmd)
	}
	if args != "just plain text" {
		t.Errorf("want full text as args, got %q", args)
	}
}

func TestParseCommand_uppercase_normalised(t *testing.T) {
	r := NewRouter("!")
	cmd, _ := r.parseCommand("!RUN goal")
	if cmd != "run" {
		t.Errorf("want cmd=run (lower-cased), got %q", cmd)
	}
}

func TestParseCommand_prefixOnly(t *testing.T) {
	r := NewRouter("!")
	cmd, args := r.parseCommand("!")
	if cmd != "" {
		t.Errorf("want empty cmd for bare prefix, got %q", cmd)
	}
	if args != "" {
		t.Errorf("want empty args for bare prefix, got %q", args)
	}
}

// ---- Router.Handle ---------------------------------------------------------

func TestRouter_Handle_registeredCommand(t *testing.T) {
	r := NewRouter("!")
	var gotChannelID, gotArgs string
	r.Register("run", func(_ context.Context, channelID, args string) {
		gotChannelID = channelID
		gotArgs = args
	})

	r.Handle(context.Background(), "MESSAGE_CREATE", &Message{
		ChannelID: "12345",
		Content:   "!run do something",
	})

	if gotChannelID != "12345" {
		t.Errorf("want channelID=12345, got %q", gotChannelID)
	}
	if gotArgs != "do something" {
		t.Errorf("want args='do something', got %q", gotArgs)
	}
}

func TestRouter_Handle_unknownCommand_callsFallback(t *testing.T) {
	r := NewRouter("!")
	var gotText string
	r.SetFallback(func(_ context.Context, _ string, text string) {
		gotText = text
	})

	r.Handle(context.Background(), "MESSAGE_CREATE", &Message{
		ChannelID: "1",
		Content:   "!unknown some args",
	})

	if gotText != "!unknown some args" {
		t.Errorf("want full text in fallback, got %q", gotText)
	}
}

func TestRouter_Handle_plainText_callsFallback(t *testing.T) {
	r := NewRouter("!")
	var gotText string
	r.SetFallback(func(_ context.Context, _ string, text string) {
		gotText = text
	})

	r.Handle(context.Background(), "MESSAGE_CREATE", &Message{
		ChannelID: "1",
		Content:   "just a plain message",
	})

	if gotText != "just a plain message" {
		t.Errorf("want fallback to receive plain text, got %q", gotText)
	}
}

func TestRouter_Handle_nilMessage_noPanic(t *testing.T) {
	r := NewRouter("!")
	r.Handle(context.Background(), "MESSAGE_CREATE", nil)
}

func TestRouter_Handle_emptyContent_noPanic(t *testing.T) {
	r := NewRouter("!")
	r.Handle(context.Background(), "MESSAGE_CREATE", &Message{ChannelID: "1", Content: ""})
}

func TestRouter_Handle_noFallback_unknownCommand_noPanic(t *testing.T) {
	r := NewRouter("!")
	// No fallback registered — must not panic on !unknown
	r.Handle(context.Background(), "MESSAGE_CREATE", &Message{
		ChannelID: "1",
		Content:   "!unknown",
	})
}

func TestRouter_Handle_caseInsensitiveRegistration(t *testing.T) {
	r := NewRouter("!")
	called := false
	r.Register("HELP", func(_ context.Context, _, _ string) { called = true })

	r.Handle(context.Background(), "MESSAGE_CREATE", &Message{
		ChannelID: "1",
		Content:   "!help",
	})

	if !called {
		t.Error("expected handler registered as HELP to be called for !help")
	}
}
