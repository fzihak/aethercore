package memory

import (
	"context"
	"testing"
)

func TestZestDBStorage_Search(t *testing.T) {
	s := NewZestDBStorage()
	ctx := context.Background()

	_ = s.Put(ctx, MemoryEntry{ID: "1", Content: "apple pie", Metadata: map[string]string{"tag": "food"}})
	_ = s.Put(ctx, MemoryEntry{ID: "2", Content: "banana bread", Metadata: map[string]string{"tag": "food"}})
	_ = s.Put(ctx, MemoryEntry{ID: "3", Content: "carrot cake", Metadata: map[string]string{"tag": "food"}})

	// 1. Keyword search
	res, err := s.Search(ctx, "banana", &SearchOptions{})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(res) != 1 || res[0].ID != "2" {
		t.Errorf("expected 1 result (ID 2), got %d", len(res))
	}

	// 2. Limit check
	res, _ = s.Search(ctx, "food", &SearchOptions{Limit: 2})
	if len(res) != 2 {
		t.Errorf("expected 2 results due to limit, got %d", len(res))
	}
}
