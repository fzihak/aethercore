package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/fzihak/aethercore/sdk"
)

// Bot is the top-level AetherCore Telegram gateway. It wires together the
// HTTP Client, Poller, Router, and Adapter into a single Start/Stop unit.
//
// Typical usage:
//
//	registry := sdk.NewModuleRegistry()
//	// ... load modules into registry ...
//	bot := telegram.NewBot(os.Getenv("TELEGRAM_TOKEN"), registry)
//	if err := bot.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
type Bot struct {
	token    string
	registry *sdk.ModuleRegistry
	log      *slog.Logger
}

// NewBot constructs a Bot for the given Telegram bot token and module registry.
func NewBot(token string, registry *sdk.ModuleRegistry) *Bot {
	opts := &slog.HandlerOptions{}
	log := slog.New(slog.NewJSONHandler(os.Stdout, opts)).With(
		slog.String("service.name", "aethercore"),
		slog.String("component", "gateway.telegram"),
	)
	return &Bot{token: token, registry: registry, log: log}
}

// Start validates the bot token with Telegram, wires the router and adapter,
// and begins the long-polling loop.  It blocks until ctx is cancelled.
//
// Returns an error only if the initial token validation (GetMe) fails.
func (b *Bot) Start(ctx context.Context) error {
	if b.token == "" {
		return fmt.Errorf("telegram: bot token must not be empty")
	}

	client := NewClient(b.token)

	// Verify the token before entering the polling loop.
	me, err := client.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("telegram: token validation failed: %w", err)
	}
	b.log.Info("telegram_bot_connected",
		slog.String("username", me.Username),
		slog.Int64("id", me.ID),
	)

	adapter := NewAdapter(client, b.registry, b.log)
	router := NewRouter()
	router.Register("start", adapter.HandleHelp)
	router.Register("help", adapter.HandleHelp)
	router.Register("run", adapter.HandleRun)
	router.Register("modules", adapter.HandleModules)

	poller := NewPoller(client, router.Handle, b.log)
	poller.Run(ctx) // blocks until ctx cancelled
	return nil
}
