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
