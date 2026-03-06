// Package memory implements a hyper-minimal in-process vector embedding store
// using pure Go. No external dependencies — cosine similarity is computed
// directly with math/bits-free float32 arithmetic.
//
// Layer 0 rule: zero external packages.
package memory

import (
	"math"
	"sort"
	"sync"
	"time"
)

// VectorEntry is a single stored embedding with its associated payload.
type VectorEntry struct {
	ID        string
	Embedding []float32
	Payload   string
	StoredAt  time.Time
}

// MemoryResult is returned by Query, sorted by descending cosine similarity.
type MemoryResult struct {
	ID      string
	Payload string
	Score   float32 // cosine similarity in [-1, 1]; higher is more similar
}

// VectorStore is a thread-safe, in-memory nearest-neighbour store.
// It performs exhaustive cosine similarity search — suitable for corpus sizes
// up to ~100 k entries at which point an index structure would be warranted.
type VectorStore struct {
	mu      sync.RWMutex
	entries map[string]*VectorEntry
}

// NewVectorStore allocates an empty VectorStore.
func NewVectorStore() *VectorStore {
	return &VectorStore{entries: make(map[string]*VectorEntry)}
}

// Store inserts or replaces the entry identified by id.
// The embedding slice is deep-copied to avoid external mutation.
func (vs *VectorStore) Store(id string, embedding []float32, payload string) {
	emb := make([]float32, len(embedding))
	copy(emb, embedding)

	vs.mu.Lock()
	vs.entries[id] = &VectorEntry{
		ID:        id,
		Embedding: emb,
		Payload:   payload,
		StoredAt:  time.Now(),
	}
	vs.mu.Unlock()
}

// Delete removes the entry with the given id.
// Returns true if the entry existed and was removed.
func (vs *VectorStore) Delete(id string) bool {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	_, ok := vs.entries[id]
	if ok {
		delete(vs.entries, id)
	}
	return ok
}

// Get returns a single entry by ID (nil if absent).
func (vs *VectorStore) Get(id string) *VectorEntry {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	e := vs.entries[id]
	if e == nil {
		return nil
	}
	// Return a shallow copy so callers cannot mutate internal state.
	cp := *e
	return &cp
}

// Len returns the current number of stored entries.
func (vs *VectorStore) Len() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.entries)
}

// Query returns the top-k entries most similar to the query embedding,
// ranked by cosine similarity in descending order.
// Entries whose embeddings differ in dimension from query are skipped.
// If topK ≤ 0 or the store is empty, an empty slice is returned.
func (vs *VectorStore) Query(embedding []float32, topK int) []MemoryResult {
	if topK <= 0 || len(embedding) == 0 {
		return nil
	}

	vs.mu.RLock()
	candidates := make([]MemoryResult, 0, len(vs.entries))
	for _, e := range vs.entries {
		if len(e.Embedding) != len(embedding) {
			continue
		}
		score := cosineSimilarity(embedding, e.Embedding)
		candidates = append(candidates, MemoryResult{
			ID:      e.ID,
			Payload: e.Payload,
			Score:   score,
		})
	}
	vs.mu.RUnlock()

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if topK > len(candidates) {
		topK = len(candidates)
	}
	return candidates[:topK]
}

// cosineSimilarity computes the cosine similarity between two equal-length
// float32 vectors. Returns 0 if either vector has zero magnitude.
func cosineSimilarity(a, b []float32) float32 {
	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}
