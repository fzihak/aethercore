package core

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestStructuredJSONLogger(t *testing.T) {
	var buf bytes.Buffer

	// Override standard output for testing
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler).With(slog.String("service.name", "aethercore"))

	// Create sub-logger
	subLogger := logger.With(slog.String("component", "test_engine"))
	subLogger.Info("Test event fired", slog.Int("latency_ms", 42))

	// Verify structural integrity
	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Structured output is NOT valid JSON: %v", err)
	}

	if output["msg"] != "Test event fired" {
		t.Fatalf("Expected msg 'Test event fired', got '%v'", output["msg"])
	}

	if output["service.name"] != "aethercore" {
		t.Fatalf("Expected service.name 'aethercore', got '%v'", output["service.name"])
	}

	if output["component"] != "test_engine" {
		t.Fatalf("Expected component 'test_engine', got '%v'", output["component"])
	}

	if int(output["latency_ms"].(float64)) != 42 {
		t.Fatalf("Expected latency_ms 42, got '%v'", output["latency_ms"])
	}
}
