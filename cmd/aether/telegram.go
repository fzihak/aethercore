package main

import (
	"context"

	"github.com/fzihak/aethercore/gateway/telegram"
	"github.com/fzihak/aethercore/sdk"
)

// handleTelegramCmd parses 'aether telegram' sub-flags and starts the Telegram
// gateway bot, blocking until SIGINT/SIGTERM is received.
func handleTelegramCmd(args []string) {
	handleGatewayCmd(
		gatewayConfig{
			name:     "telegram",
			envKey:   "TELEGRAM_TOKEN",
			commands: "/start, /help, /run, and /modules",
		},
		args,
		func(ctx context.Context, token string, registry *sdk.ModuleRegistry) error {
			return telegram.NewBot(token, registry).Start(ctx)
		},
	)
}
