package core

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// BenchmarkEventLoopAllocation measures the memory footprint of a full ephemeral task lifecycle.
// The goal is strictly 0 B/op and 0 allocs/op for the Task and Result routing logic.
func BenchmarkEventLoopAllocation(b *testing.B) {
	// Initialize a silent logger so stdout doesn't become the bottleneck
	InitLogger(999) // Way above Error so it drops everything

	adapter := &MockLLMAdapter{}
	// Large queue to ensure we don't block during the pure allocation bench
	engine := NewEngine(adapter, 4, b.N+100)
	engine.Start()

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		t := engine.GetTask()
		t.ID = "bench_task"
		t.System = "System"
		t.Input = "Input"
		t.CreatedAt = time.Now()

		engine.Submit(t)
	}

	for i := 0; i < b.N; i++ {
		res := <-engine.Results()
		engine.RecycleResult(res)
	}

	b.StopTimer()
	engine.Stop()

	runtime.ReadMemStats(&m2)
	allocs := m2.Mallocs - m1.Mallocs
	fmt.Printf("\n[BENCHMARK] Total Allocations over %d ops: %d (Target: 0 structural allocs outside GC jitter)\n", b.N, allocs)
}

// BenchmarkAdapterLatency measures the absolute overhead introduced by the Engine logic
// wrapping a sub-millisecond LLM call.
func BenchmarkAdapterLatency(b *testing.B) {
	InitLogger(999)

	adapter := &MockLLMAdapter{}
	engine := NewEngine(adapter, 1, 10)
	engine.Start()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		t := engine.GetTask()
		t.ID = "bench_task"

		engine.Submit(t)

		res := <-engine.Results()
		engine.RecycleResult(res)
	}

	b.StopTimer()
	engine.Stop()
}
