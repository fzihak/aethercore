package memory

import (
	"math"
	"testing"
)

func TestStore_StoreAndLen(t *testing.T) {
	vs := NewVectorStore()
	if vs.Len() != 0 {
		t.Fatalf("expected 0 entries, got %d", vs.Len())
	}

	vs.Store("e1", []float32{1, 0, 0}, "first")
	vs.Store("e2", []float32{0, 1, 0}, "second")

	if vs.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", vs.Len())
	}
}

func TestStore_Upsert(t *testing.T) {
	vs := NewVectorStore()
	vs.Store("doc", []float32{1, 0}, "original")
	vs.Store("doc", []float32{0, 1}, "updated")

	if vs.Len() != 1 {
		t.Fatalf("upsert should keep one entry, got %d", vs.Len())
	}
	e := vs.Get("doc")
	if e == nil {
		t.Fatal("entry not found after upsert")
	}
	if e.Payload != "updated" {
		t.Errorf("expected payload 'updated', got '%s'", e.Payload)
	}
}

func TestStore_Delete(t *testing.T) {
	vs := NewVectorStore()
	vs.Store("del-me", []float32{1, 0}, "bye")

	removed := vs.Delete("del-me")
	if !removed {
		t.Error("expected Delete to return true")
	}
	if vs.Len() != 0 {
		t.Errorf("expected 0 entries after delete, got %d", vs.Len())
	}
	if vs.Delete("del-me") {
		t.Error("second Delete should return false")
	}
}

func TestQuery_TopK(t *testing.T) {
	vs := NewVectorStore()
	vs.Store("cat", []float32{1, 0, 0}, "feline")
	vs.Store("dog", []float32{0.9, 0.1, 0}, "canine")
	vs.Store("fish", []float32{0, 0, 1}, "aquatic")

	// Query closest to [1, 0, 0] — should rank cat first, then dog.
	results := vs.Query([]float32{1, 0, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "cat" {
		t.Errorf("expected 'cat' at rank 0, got '%s'", results[0].ID)
	}
	if results[1].ID != "dog" {
		t.Errorf("expected 'dog' at rank 1, got '%s'", results[1].ID)
	}
}

func TestQuery_EmptyStore(t *testing.T) {
	vs := NewVectorStore()
	results := vs.Query([]float32{1, 0}, 5)
	if len(results) != 0 {
		t.Errorf("expected empty results for empty store, got %d", len(results))
	}
}

func TestQuery_DimensionMismatch(t *testing.T) {
	vs := NewVectorStore()
	vs.Store("3d", []float32{1, 0, 0}, "three-dimensional")

	// Query with wrong dimension — should skip the entry.
	results := vs.Query([]float32{1, 0}, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results from mismatch, got %d", len(results))
	}
}

func TestCosineSimilarity_KnownValues(t *testing.T) {
	// Identical vectors → similarity = 1.
	a := []float32{3, 4}
	sim := cosineSimilarity(a, a)
	if math.Abs(float64(sim)-1.0) > 1e-5 {
		t.Errorf("identical vectors: expected 1.0, got %f", sim)
	}

	// Orthogonal vectors → similarity = 0.
	ox := []float32{1, 0}
	oy := []float32{0, 1}
	sim = cosineSimilarity(ox, oy)
	if math.Abs(float64(sim)) > 1e-5 {
		t.Errorf("orthogonal vectors: expected 0.0, got %f", sim)
	}

	// Opposite vectors → similarity = -1.
	neg := []float32{-1, 0}
	sim = cosineSimilarity(ox, neg)
	if math.Abs(float64(sim)+1.0) > 1e-5 {
		t.Errorf("opposite vectors: expected -1.0, got %f", sim)
	}
}

func TestStore_IsolatesEmbedding(t *testing.T) {
	vs := NewVectorStore()
	emb := []float32{1, 2, 3}
	vs.Store("iso", emb, "isolated")

	// Mutate original — store must be unaffected.
	emb[0] = 99
	e := vs.Get("iso")
	if e == nil {
		t.Fatal("entry not found")
	}
	if e.Embedding[0] != 1 {
		t.Errorf("store did not deep-copy embedding; expected 1, got %v", e.Embedding[0])
	}
}
