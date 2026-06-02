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
	mu    sync.RWMutex
	data  map[string]MemoryEntry
	index map[string]map[string]struct{} // Trigram index for fast searching
}

// NewZestDBStorage initializes a new instance of the ZestDB adapter.
func NewZestDBStorage() *ZestDBStorage {
	return &ZestDBStorage{
		data:  make(map[string]MemoryEntry),
		index: make(map[string]map[string]struct{}),
	}
}

// Put saves a new entry to the in-memory persistence layer.
func (s *ZestDBStorage) Put(ctx context.Context, entry MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		return errors.New("memory_entry_id_required")
	}

	// Remove old index entries if updating
	if oldEntry, exists := s.data[entry.ID]; exists {
		s.removeIndex(oldEntry)
	}

	s.data[entry.ID] = entry
	s.addIndex(entry)

	return nil
}

func (s *ZestDBStorage) addIndex(entry MemoryEntry) {
	trigrams := extractTrigrams(entry.Content)
	for _, val := range entry.Metadata {
		trigrams = append(trigrams, extractTrigrams(val)...)
	}

	for _, tg := range trigrams {
		if s.index[tg] == nil {
			s.index[tg] = make(map[string]struct{})
		}
		s.index[tg][entry.ID] = struct{}{}
	}
}

func (s *ZestDBStorage) removeIndex(entry MemoryEntry) {
	trigrams := extractTrigrams(entry.Content)
	for _, val := range entry.Metadata {
		trigrams = append(trigrams, extractTrigrams(val)...)
	}

	for _, tg := range trigrams {
		if ids, ok := s.index[tg]; ok {
			delete(ids, entry.ID)
			if len(ids) == 0 {
				delete(s.index, tg)
			}
		}
	}
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

	if entry, exists := s.data[id]; exists {
		s.removeIndex(entry)
		delete(s.data, id)
	}

	return nil
}

// getCandidates returns the set of candidate IDs from the trigram index.
func (s *ZestDBStorage) getCandidates(queryTrigrams []string) map[string]struct{} {
	var candidateIDs map[string]struct{}
	for i, tg := range queryTrigrams {
		ids, ok := s.index[tg]
		if !ok || len(ids) == 0 {
			// A trigram is missing, so the query cannot be found
			return make(map[string]struct{})
		}

		if i == 0 {
			candidateIDs = make(map[string]struct{}, len(ids))
			for id := range ids {
				candidateIDs[id] = struct{}{}
			}
		} else {
			nextMatched := make(map[string]struct{})
			for id := range candidateIDs {
				if _, exists := ids[id]; exists {
					nextMatched[id] = struct{}{}
				}
			}
			candidateIDs = nextMatched
		}
	}
	return candidateIDs
}

// Search queries the in-memory persistence layer for entries matching the criteria.
//
//nolint:gocognit // This method handles indexing and fallback logic cleanly
func (s *ZestDBStorage) Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []MemoryEntry
	queryTrigrams := extractTrigrams(query)

	// Determine if we should use the index
	useIndex := query != "" && len(queryTrigrams) > 0 && len([]rune(query)) >= 3

	if useIndex {
		candidateIDs := s.getCandidates(queryTrigrams)
		for id := range candidateIDs {
			entry, exists := s.data[id]
			if !exists {
				continue
			}
			if s.matches(&entry, query, &opts) {
				results = append(results, entry)
				if opts.Limit > 0 && len(results) >= opts.Limit {
					break
				}
			}
		}
	} else {
		for _, entry := range s.data {
			if s.matches(&entry, query, &opts) {
				results = append(results, entry)
				if opts.Limit > 0 && len(results) >= opts.Limit {
					break
				}
			}
		}
	}

	return results, nil
}

// matches performs the actual match logic to eliminate false positives and check tags.
func (s *ZestDBStorage) matches(entry *MemoryEntry, query string, opts *SearchOptions) bool {
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
		return false
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
			return false
		}
	}

	return true
}

// Close releases any resources held by the storage adapter.
func (s *ZestDBStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = nil
	s.index = nil
	return nil
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// extractTrigrams generates lowercase trigrams from a string for indexing.
func extractTrigrams(s string) []string {
	s = strings.ToLower(s)
	runes := []rune(s)

	if len(runes) == 0 {
		return nil
	}

	if len(runes) < 3 {
		return []string{string(runes)}
	}

	var res []string
	for i := 0; i <= len(runes)-3; i++ {
		res = append(res, string(runes[i:i+3]))
	}
	return res
}
