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
	"github.com/fzihak/aethercore/sdk"
)

// gatewayConfig holds the common configuration for launching a gateway adapter.
type gatewayConfig struct {
	name     string // "telegram" or "discord"
	envKey   string // e.g. "TELEGRAM_TOKEN"
	commands string // e.g. "/start, /help, /run, and /modules"
	prefix   string // command prefix description
}

// gatewayStarter is a function that takes a bot token, registry, and blocking
// context, then runs the gateway bot until the context is cancelled.
type gatewayStarter func(ctx context.Context, token string, registry *sdk.ModuleRegistry) error

// handleGatewayCmd is the shared backbone for the Telegram and Discord CLI
// commands. It parses the --token flag, resolves environment fallback, sets up
// a signal-aware context, and delegates to the platform-specific starter.
func handleGatewayCmd(cfg gatewayConfig, args []string, start gatewayStarter) {
	fs := flag.NewFlagSet(cfg.name, flag.ContinueOnError)
	token := fs.String("token", "", fmt.Sprintf("%s bot token (or set %s env var) [required]", cfg.name, cfg.envKey))

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: aether %s --token <BOT_TOKEN>\n\n", cfg.name)
		fmt.Fprintf(os.Stderr, "Starts the AetherCore %s gateway.\n", cfg.name)
		fmt.Fprintf(os.Stderr, "The bot responds to %s.\n\n", cfg.commands)
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment:\n")
		fmt.Fprintf(os.Stderr, "  %s   Alternative to --token flag\n", cfg.envKey)
	}

	if err := fs.Parse(args); err != nil {
		core.Logger().Error(cfg.name+"_parse_flags_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	botToken := *token
	if botToken == "" {
		botToken = os.Getenv(cfg.envKey)
	}
	if botToken == "" {
		fmt.Fprintf(os.Stderr, "Error: %s bot token required (--token or %s env var)\n", cfg.name, cfg.envKey)
		fs.Usage()
		os.Exit(1)
	}

	registry := sdk.NewModuleRegistry()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	core.Logger().Info(cfg.name + "_gateway_starting")

	err := start(ctx, botToken, registry)
	stop()
	if err != nil {
		core.Logger().Error(cfg.name+"_gateway_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	core.Logger().Info(cfg.name + "_gateway_stopped")
}
