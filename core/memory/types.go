package memory

import (
	"context"
	"time"
)

// MemoryEntry represents a single unit of persistent episodic or semantic memory.
type MemoryEntry struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
}

// SearchOptions defines filters for querying the memory store.
type SearchOptions struct {
	Limit     int
	Tags      []string
	MinScore  float64
	StartTime time.Time
	EndTime   time.Time
}

// Storage defines the interface for low-level memory persistence (e.g. ZestDB).
type Storage interface {
	Put(ctx context.Context, entry MemoryEntry) error
	Get(ctx context.Context, id string) (MemoryEntry, error)
	Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryEntry, error)
	Delete(ctx context.Context, id string) error
	Close() error
}
