package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/fzihak/aethercore/core/llm"
)

// MemoryEngine manages the orchestration of episodic and persistent storage.
// It handles context retention, summarization, and retrieval-augmented generation (RAG) precursors.
type MemoryEngine struct {
	storage      Storage
	shortTermMem []llm.Message
	maxShortTerm int
}

// NewMemoryEngine creates a new management layer for ephemeral and persistent memories.
func NewMemoryEngine(storage Storage, maxShortTerm int) *MemoryEngine {
	return &MemoryEngine{
		storage:      storage,
		shortTermMem: make([]llm.Message, 0),
		maxShortTerm: maxShortTerm,
	}
}

// Record saves a message to both ephemeral short-term memory and persistent episodic storage.
func (e *MemoryEngine) Record(ctx context.Context, msg llm.Message) error {
	e.shortTermMem = append(e.shortTermMem, msg)
	if len(e.shortTermMem) > e.maxShortTerm {
		e.shortTermMem = e.shortTermMem[1:] // Simple FIFO eviction for Layer 0
	}

	entry := MemoryEntry{
		ID:        fmt.Sprintf("mem_%d", time.Now().UnixNano()),
		Content:   msg.Content,
		Metadata:  map[string]string{"role": msg.Role},
		CreatedAt: time.Now(),
	}

	return e.storage.Put(ctx, entry)
}

// Recall retrieves short-term memory and relevant long-term memories for a given query.
func (e *MemoryEngine) Recall(ctx context.Context, query string) ([]llm.Message, error) {
	// 1. Start with short-term memory (most recent context)
	combined := make([]llm.Message, len(e.shortTermMem))
	copy(combined, e.shortTermMem)

	// 2. Fetch relevant long-term memories via storage search
	// In Layer 0, we use simple keyword matching for RAG-lite behavior.
	entries, err := e.storage.Search(ctx, query, SearchOptions{Limit: 3})
	if err != nil {
		return combined, fmt.Errorf("long_term_recall_failed: %w", err)
	}

	// 3. Inject long-term memories as system context "reminders"
	for _, entry := range entries {
		combined = append(combined, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("[Memory Recall] %s", entry.Content),
		})
	}

	return combined, nil
}

// Summarize performs a context compression by merging older short-term memories.
// In Layer 0, this is a placeholder that simulates token-limit-driven summarization.
func (e *MemoryEngine) Summarize(ctx context.Context) error {
	if len(e.shortTermMem) <= 3 {
		return nil
	}

	// Heuristic: Take the oldest half and "compress" them into a single system message
	summaryMsg := llm.Message{
		Role:    "system",
		Content: "[Context Summary] The conversation began with objective initialization and tool discovery.",
	}

	e.shortTermMem = append([]llm.Message{summaryMsg}, e.shortTermMem[len(e.shortTermMem)/2:]...)
	return nil
}
