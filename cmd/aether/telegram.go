package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/fzihak/aethercore/core"
	"github.com/fzihak/aethercore/gateway/telegram"
	"github.com/fzihak/aethercore/sdk"
)

// handleTelegramCmd parses 'aether telegram' sub-flags and starts the Telegram
// gateway bot, blocking until SIGINT/SIGTERM is received.
func handleTelegramCmd(args []string) {
	tgCmd := flag.NewFlagSet("telegram", flag.ContinueOnError)
	token := tgCmd.String("token", "", "Telegram bot token (or set TELEGRAM_TOKEN env var) [required]")
	tgCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: aether telegram --token <BOT_TOKEN>\n\n")
		fmt.Fprintf(os.Stderr, "Starts the AetherCore Telegram gateway.\n")
		fmt.Fprintf(os.Stderr, "The bot responds to /start, /help, /run, and /modules.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		tgCmd.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment:\n")
		fmt.Fprintf(os.Stderr, "  TELEGRAM_TOKEN   Alternative to --token flag\n")
	}

	if err := tgCmd.Parse(args); err != nil {
		core.Logger().Error("telegram_parse_flags_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// --token flag takes precedence; fall back to environment variable.
	botToken := *token
	if botToken == "" {
		botToken = os.Getenv("TELEGRAM_TOKEN")
	}
	if botToken == "" {
		fmt.Fprintln(os.Stderr, "Error: Telegram bot token required (--token or TELEGRAM_TOKEN env var)")
		tgCmd.Usage()
		os.Exit(1)
	}

	// Build an empty module registry; callers can pre-load modules before
	// handing off to handleTelegramCmd by expanding this function in future.
	registry := sdk.NewModuleRegistry()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	core.Logger().Info("telegram_gateway_starting")

	bot := telegram.NewBot(botToken, registry)
	err := bot.Start(ctx)
	stop() // release signal resources regardless of outcome
	if err != nil {
		core.Logger().Error("telegram_gateway_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	core.Logger().Info("telegram_gateway_stopped")
}
