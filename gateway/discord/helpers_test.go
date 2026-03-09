package discord

import (
	"io"
	"log/slog"
)

// newTestLogger returns a silent slog.Logger that discards all output.
// Used by unit tests to suppress log noise without nil-checks in production code.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
