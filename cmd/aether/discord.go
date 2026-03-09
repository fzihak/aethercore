package main

import (
	"context"

	"github.com/fzihak/aethercore/gateway/discord"
	"github.com/fzihak/aethercore/sdk"
)

// handleDiscordCmd parses 'aether discord' sub-flags and starts the Discord
// gateway bot, blocking until SIGINT/SIGTERM is received.
func handleDiscordCmd(args []string) {
	handleGatewayCmd(
		gatewayConfig{
			name:     "discord",
			envKey:   "DISCORD_TOKEN",
			commands: "!start, !help, !run, and !modules",
		},
		args,
		func(ctx context.Context, token string, registry *sdk.ModuleRegistry) error {
			return discord.NewBot(token, registry).Start(ctx)
		},
	)
}
