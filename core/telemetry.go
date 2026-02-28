package core

import (
	"fmt"
	"time"
)

// Telemetry captures structured execution metrics for the AetherCore engine.
// It is designed to have zero heap allocations during the critical boot path.
type Telemetry struct {
	StartTime time.Time
}

// ZeroLatency string literal constant for uninitialized timer scenarios.
const ZeroLatency = "0ms"

// Global engine telemetry instance. Initialized at absolute binary start.
var EngineTelemetry Telemetry

// InitTelemetry records the absolute nanosecond the process started.
// This must be called incredibly early in the main() function.
func InitTelemetry() {
	EngineTelemetry.StartTime = time.Now()
}

// BootLatency returns the elapsed time since InitTelemetry was called.
func BootLatency() time.Duration {
	if EngineTelemetry.StartTime.IsZero() {
		return 0
	}
	return time.Since(EngineTelemetry.StartTime)
}

// FormatBootLatency returns a human-readable string of the boot latency.
func FormatBootLatency() string {
	lat := BootLatency()
	if lat == 0 {
		return ZeroLatency
	}
	// For execution times under 1ms, format to microseconds for precision
	if lat < time.Millisecond {
		return fmt.Sprintf("%dµs", lat.Microseconds())
	}
	return fmt.Sprintf("%dms", lat.Milliseconds())
}
