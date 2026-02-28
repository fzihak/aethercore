package core

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

// Inviolable Rule: Layer 0 strictly uses Go stdlib ONLY.

var (
	// Log is the global singleton logger for the AetherCore Layer 0 Kernel.
	Log     *slog.Logger
	logOnce sync.Once
)

// InitLogger boots the absolute zero-allocation JSON telemetry engine.
// It maps natively to standard OpenTelemetry semantic conventions.
func InitLogger(level slog.Level) {
	logOnce.Do(func() {
		opts := &slog.HandlerOptions{
			Level: level,
			// Replace time key with standard otel specification
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					a.Key = "timestamp"
				}
				if a.Key == slog.MessageKey {
					a.Key = "msg" // shorter for log shippers
				}
				return a
			},
		}

		// Strictly enforce JSON format on os.Stdout for parsing
		handler := slog.NewJSONHandler(os.Stdout, opts)

		// Pre-attach the global instance tags
		Log = slog.New(handler).With(
			slog.String("service.name", "aethercore"),
			slog.String("service.version", "v0.1.0"), // will inject build tag later
		)

		// Re-map the standard 'log' package to use this structured logger as well
		slog.SetDefault(Log)
	})
}

// Logger returns the global instance, primarily used for testing or specific package overrides.
func Logger() *slog.Logger {
	if Log == nil {
		InitLogger(slog.LevelInfo)
	}
	return Log
}

// WithComponent creates a child logger annotated with a specific subsystem
func WithComponent(component string) *slog.Logger {
	return Logger().With(slog.String("component", component))
}

// WithTask creates a child logger strictly bound to a specific ephemeral task ID
func WithTask(ctx context.Context, taskID string) *slog.Logger {
	return Logger().With(
		slog.String("task_id", taskID),
	)
}
