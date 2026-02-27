package core

import (
	"testing"
	"time"
)

func TestTelemetryLatency(t *testing.T) {
	InitTelemetry()
	
	if EngineTelemetry.StartTime.IsZero() {
		t.Fatal("Expected StartTime to be populated")
	}

	// Artificial sleep to test latency calculation
	time.Sleep(2 * time.Millisecond)

	latency := BootLatency()
	if latency < 2*time.Millisecond {
		t.Fatalf("Expected latency >= 2ms, got %v", latency)
	}

	formatted := FormatBootLatency()
	if formatted == "0ms" || formatted == "" {
		t.Fatalf("Expected valid formatted string, got %q", formatted)
	}
}

func TestTelemetryUninitialized(t *testing.T) {
	// Reset global state for this test
	EngineTelemetry = Telemetry{}

	if BootLatency() != 0 {
		t.Fatal("Expected 0 latency when uninitialized")
	}

	if FormatBootLatency() != "0ms" {
		t.Fatal("Expected '0ms' string when uninitialized")
	}
}
