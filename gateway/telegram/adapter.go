package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fzihak/aethercore/sdk"
)

// Adapter bridges Telegram messages into the AetherCore sdk.ModuleRegistry.
//
// When a user sends "/run <goal>" the adapter:
//  1. Wraps the goal in an sdk.ModuleTask
//  2. Dispatches the task to every module in the registry via HandleTask
//  3. Concatenates all results and sends the reply back to the same chat
//
// All Telegram replies are sent using the Adapter's Client reference so
// they stay in the same HTTP connection pool.
type Adapter struct {
	client   *Client
	registry *sdk.ModuleRegistry
	log      *slog.Logger
}

// NewAdapter constructs an Adapter wired to the given registry and client.
func NewAdapter(client *Client, registry *sdk.ModuleRegistry, log *slog.Logger) *Adapter {
	return &Adapter{client: client, registry: registry, log: log}
}

// HandleRun is the CommandHandler for the "/run" bot command.
// It converts the argument string into an sdk.ModuleTask, fans it out to
// every loaded module, collects results, and replies via sendMessage.
func (a *Adapter) HandleRun(ctx context.Context, chatID int64, goal string) {
	if strings.TrimSpace(goal) == "" {
		a.reply(ctx, chatID, "Usage: `/run <your goal>`")
		return
	}

	task := &sdk.ModuleTask{
		ID:    fmt.Sprintf("tg-%d-%d", chatID, time.Now().UnixNano()),
		Input: goal,
		Metadata: map[string]string{
			"source":  "telegram",
			"chat_id": fmt.Sprintf("%d", chatID),
		},
	}

	manifests := a.registry.Manifests()
	if len(manifests) == 0 {
		a.reply(ctx, chatID, "⚠️ No modules are currently loaded.")
		return
	}

	var sb strings.Builder
	for _, mf := range manifests {
		mod, err := a.registry.Get(mf.Name)
		if err != nil {
			a.log.Error("telegram_adapter_get_module_failed",
				slog.String("module", mf.Name),
				slog.String("error", err.Error()),
			)
			continue
		}

		result, err := mod.HandleTask(ctx, task)
		if err != nil {
			a.log.Error("telegram_adapter_task_failed",
				slog.String("module", mf.Name),
				slog.String("error", err.Error()),
			)
			sb.WriteString(fmt.Sprintf("*%s* ❌ error: %s\n\n", mf.Name, err.Error()))
			continue
		}
		sb.WriteString(fmt.Sprintf("*%s*\n%s\n\n", mf.Name, result.Output))
	}

	a.reply(ctx, chatID, strings.TrimSpace(sb.String()))
}

// HandleHelp is the CommandHandler for the "/help" and "/start" bot commands.
func (a *Adapter) HandleHelp(ctx context.Context, chatID int64, _ string) {
	manifests := a.registry.Manifests()
	var sb strings.Builder
	sb.WriteString("*AetherCore* — Minimal Agent Kernel\n\n")
	sb.WriteString("*Commands:*\n")
	sb.WriteString("`/run <goal>` — dispatch a task to all loaded modules\n")
	sb.WriteString("`/modules` — list loaded modules\n")
	sb.WriteString("`/help` — show this message\n")

	if len(manifests) > 0 {
		sb.WriteString("\n*Loaded modules:*\n")
		for _, mf := range manifests {
			sb.WriteString(fmt.Sprintf("• *%s* v%s — %s\n", mf.Name, mf.Version, mf.Description))
		}
	}

	a.reply(ctx, chatID, sb.String())
}

// HandleModules is the CommandHandler for the "/modules" bot command.
func (a *Adapter) HandleModules(ctx context.Context, chatID int64, _ string) {
	manifests := a.registry.Manifests()
	if len(manifests) == 0 {
		a.reply(ctx, chatID, "No modules loaded.")
		return
	}

	var sb strings.Builder
	sb.WriteString("*Loaded modules:*\n\n")
	for _, mf := range manifests {
		sb.WriteString(fmt.Sprintf("*%s* v%s\n_%s_\nAuthor: %s\n\n",
			mf.Name, mf.Version, mf.Description, mf.Author))
	}
	a.reply(ctx, chatID, strings.TrimSpace(sb.String()))
}

// reply is a safe wrapper around client.SendMessage that logs errors instead
// of propagating them — Telegram network errors must not crash the adapter.
func (a *Adapter) reply(ctx context.Context, chatID int64, text string) {
	if _, err := a.client.SendMessage(ctx, chatID, text); err != nil {
		a.log.Error("telegram_send_message_failed",
			slog.Int64("chat_id", chatID),
			slog.String("error", err.Error()),
		)
	}
}
