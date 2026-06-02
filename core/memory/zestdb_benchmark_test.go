package memory

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkZestDBStorage_Search(b *testing.B) {
	s := NewZestDBStorage()
	ctx := context.Background()

	// Populate with dummy data
	for i := 0; i < 10000; i++ {
		_ = s.Put(ctx, MemoryEntry{
			ID:      fmt.Sprintf("%d", i),
			Content: fmt.Sprintf("this is some dummy content about %d and other things like banana or apple", i),
			Metadata: map[string]string{
				"tag": fmt.Sprintf("category_%d", i%10),
			},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Search(ctx, "banana", SearchOptions{Limit: 10})
	}
}

func BenchmarkZestDBStorage_Search_Miss(b *testing.B) {
	s := NewZestDBStorage()
	ctx := context.Background()

	for i := 0; i < 10000; i++ {
		_ = s.Put(ctx, MemoryEntry{
			ID:      fmt.Sprintf("%d", i),
			Content: fmt.Sprintf("this is some dummy content about %d and other things like banxna or apple", i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Search(ctx, "missingword", SearchOptions{Limit: 10})
	}
}
