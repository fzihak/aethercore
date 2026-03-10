package memory

import (
	"sync"
)

// ZestDBStorage is a lightweight, high-performance persistence layer.
// In this Layer 0 implementation, it uses a thread-safe map as a proxy for the Rust-based ZestDB.
type ZestDBStorage struct {
	mu   sync.RWMutex
	data map[string]MemoryEntry
}

// NewZestDBStorage initializes a new instance of the ZestDB adapter.
func NewZestDBStorage() *ZestDBStorage {
	return &ZestDBStorage{
		data: make(map[string]MemoryEntry),
	}
}
