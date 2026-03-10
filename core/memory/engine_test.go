package memory

import (
	"context"
	"testing"
	"time"

	"github.com/fzihak/aethercore/core/llm"
)

func TestMemoryEngine_Record(t *testing.T) {
	storage := NewZestDBStorage()
	engine := NewMemoryEngine(storage, 5)

	msg := llm.Message{Role: "user", Content: "hello world"}
	err := engine.Record(context.Background(), msg)
	if err != nil {
		t.Fatalf("failed to record memory: %v", err)
	}

	if len(engine.shortTermMem) != 1 {
		t.Errorf("expected 1 msg in short-term, got %d", len(engine.shortTermMem))
	}
}

func TestMemoryEngine_Recall(t *testing.T) {
	storage := NewZestDBStorage()
	engine := NewMemoryEngine(storage, 5)

	ctx := context.Background()
	_ = engine.Record(ctx, llm.Message{Role: "user", Content: "AetherCore is a security sandbox."})
	time.Sleep(1 * time.Millisecond) // Ensure unique timestamp ID
	_ = engine.Record(ctx, llm.Message{Role: "assistant", Content: "Understood."})

	messages, err := engine.Recall(ctx, "security")
	if err != nil {
		t.Fatalf("failed to recall: %v", err)
	}

	// Should have 2 short-term + some long-term (if matched)
	if len(messages) < 2 {
		t.Errorf("expected at least 2 messages, got %d", len(messages))
	}

	found := false
	for _, m := range messages {
		if m.Role == "system" && (len(m.Content) > 15 && m.Content[:15] == "[Memory Recall]") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected long-term memory recall but not found")
	}
}

func TestMemoryEngine_Summarize(t *testing.T) {
	storage := NewZestDBStorage()
	engine := NewMemoryEngine(storage, 5)

	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_ = engine.Record(ctx, llm.Message{Role: "user", Content: "filler message"})
	}

	if len(engine.shortTermMem) != 4 {
		t.Fatalf("expected 4 messages before summarization, got %d", len(engine.shortTermMem))
	}

	err := engine.Summarize(ctx)
	if err != nil {
		t.Fatalf("summarization failed: %v", err)
	}

	// After summarization (4/2 + 1) = 3 messages
	if len(engine.shortTermMem) != 3 {
		t.Errorf("expected 3 messages after summarization, got %d", len(engine.shortTermMem))
	}

	if engine.shortTermMem[0].Role != "system" {
		t.Errorf("expected first message to be system summary, got %s", engine.shortTermMem[0].Role)
	}
}
