package memory

import (
	"context"
	"errors"
	"strings"
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

// Put saves a new entry to the in-memory persistence layer.
func (s *ZestDBStorage) Put(ctx context.Context, entry MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		return errors.New("memory_entry_id_required")
	}

	s.data[entry.ID] = entry
	return nil
}

// Get retrieves a specific memory entry by its ID.
func (s *ZestDBStorage) Get(ctx context.Context, id string) (MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.data[id]
	if !exists {
		return MemoryEntry{}, errors.New("memory_entry_not_found")
	}
	return entry, nil
}

// Delete removes a specific memory entry from the persistence layer.
func (s *ZestDBStorage) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, id)
	return nil
}

// Search queries the in-memory persistence layer for entries matching the criteria.
func (s *ZestDBStorage) Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, entry := range s.data {
		// 1. Keyword match in content
		match := query == "" || containsIgnoreCase(entry.Content, query)

		// 2. Keyword match in metadata values
		if !match && query != "" {
			for _, val := range entry.Metadata {
				if containsIgnoreCase(val, query) {
					match = true
					break
				}
			}
		}

		if !match {
			continue
		}

		// 3. Optional tag filter (must match specific keys)
		if len(opts.Tags) > 0 {
			tagMatch := false
			for _, tag := range opts.Tags {
				if _, exists := entry.Metadata[tag]; exists {
					tagMatch = true
					break
				}
			}
			if !tagMatch {
				continue
			}
		}

		results = append(results, entry)
		if opts.Limit > 0 && len(results) >= opts.Limit {
			break
		}
	}

	return results, nil
}

// Close releases any resources held by the storage adapter.
func (s *ZestDBStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = nil
	return nil
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
