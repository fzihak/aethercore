package memory

import (
	"github.com/fzihak/aethercore/core/llm"
)

// MemoryEngine manages the orchestration of episodic and persistent storage.
// It handles context retention, summarization, and retrieval-augmented generation (RAG) precursors.
type MemoryEngine struct {
	storage      Storage
	shortTermMem []llm.Message
	maxShortTerm int
}
