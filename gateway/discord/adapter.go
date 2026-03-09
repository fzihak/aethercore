package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fzihak/aethercore/sdk"
)

// Adapter bridges Discord messages into the AetherCore sdk.ModuleRegistry.
//
// When a user sends "!run <goal>" the adapter:
//  1. Wraps the goal in an sdk.ModuleTask
//  2. Dispatches the task to every loaded module via HandleTask
//  3. Concatenates all results and sends the reply to the same channel
//
// All Discord replies are sent through the Adapter's Client so they share
// the same HTTP connection pool.
type Adapter struct {
	client   *Client
	registry *sdk.ModuleRegistry
	log      *slog.Logger
}

// NewAdapter constructs an Adapter wired to the given registry and REST client.
func NewAdapter(client *Client, registry *sdk.ModuleRegistry, log *slog.Logger) *Adapter {
	return &Adapter{client: client, registry: registry, log: log}
}

// HandleRun is the CommandHandler for the "!run" bot command.
// It converts the argument string into an sdk.ModuleTask, fans it out to every
// loaded module, collects results, and posts the reply to the Discord channel.
func (a *Adapter) HandleRun(ctx context.Context, channelID, goal string) {
	if strings.TrimSpace(goal) == "" {
		a.reply(ctx, channelID, "Usage: `!run <your goal>`")
		return
	}

	task := &sdk.ModuleTask{
		ID:    fmt.Sprintf("dc-%s-%d", channelID, time.Now().UnixNano()),
		Input: goal,
		Metadata: map[string]string{
			"source":     "discord",
			"channel_id": channelID,
		},
	}

	manifests := a.registry.Manifests()
	if len(manifests) == 0 {
		a.reply(ctx, channelID, "⚠️ No modules are currently loaded.")
		return
	}

	var sb strings.Builder
	for _, mf := range manifests {
		mod, err := a.registry.Get(mf.Name)
		if err != nil {
			a.log.Error("discord_adapter_get_module_failed",
				slog.String("module", mf.Name),
				slog.String("error", err.Error()),
			)
			continue
		}

		result, err := mod.HandleTask(ctx, task)
		if err != nil {
			a.log.Error("discord_adapter_task_failed",
				slog.String("module", mf.Name),
				slog.String("error", err.Error()),
			)
			sb.WriteString(fmt.Sprintf("**%s** ❌ error: %s\n\n", mf.Name, err.Error()))
			continue
		}
		sb.WriteString(fmt.Sprintf("**%s**\n%s\n\n", mf.Name, result.Output))
	}

	a.reply(ctx, channelID, strings.TrimSpace(sb.String()))
}

// HandleHelp is the CommandHandler for the "!help" and "!start" commands.
func (a *Adapter) HandleHelp(ctx context.Context, channelID, _ string) {
	manifests := a.registry.Manifests()
	var sb strings.Builder
	sb.WriteString("**AetherCore** — Minimal Agent Kernel\n\n")
	sb.WriteString("**Commands:**\n")
	sb.WriteString("`!run <goal>` — dispatch a task to all loaded modules\n")
	sb.WriteString("`!modules` — list loaded modules\n")
	sb.WriteString("`!help` — show this message\n")

	if len(manifests) > 0 {
		sb.WriteString("\n**Loaded modules:**\n")
		for _, mf := range manifests {
			sb.WriteString(fmt.Sprintf("• **%s** v%s — %s\n", mf.Name, mf.Version, mf.Description))
		}
	}
	a.reply(ctx, channelID, sb.String())
}

// HandleModules is the CommandHandler for the "!modules" command.
func (a *Adapter) HandleModules(ctx context.Context, channelID, _ string) {
	manifests := a.registry.Manifests()
	if len(manifests) == 0 {
		a.reply(ctx, channelID, "No modules are currently loaded.")
		return
	}

	var sb strings.Builder
	sb.WriteString("**Loaded modules:**\n")
	for _, mf := range manifests {
		sb.WriteString(fmt.Sprintf("• **%s** v%s — %s\n", mf.Name, mf.Version, mf.Description))
	}
	a.reply(ctx, channelID, sb.String())
}

// reply sends text to the Discord channel, logging any delivery failure.
func (a *Adapter) reply(ctx context.Context, channelID, text string) {
	if _, err := a.client.SendMessage(ctx, channelID, text); err != nil {
		a.log.Error("discord_send_message_failed",
			slog.String("channel_id", channelID),
			slog.String("error", err.Error()),
		)
	}
}
