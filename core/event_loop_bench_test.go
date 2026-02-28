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

	for range b.N {
		t := engine.GetTask()
		t.ID = "bench_task"
		t.System = "System"
		t.Input = "Input"
		t.CreatedAt = time.Now()

		if err := engine.Submit(t); err != nil {
			b.Fatalf("engine.Submit failed during benchmark: %v", err)
		}
	}

	for range b.N {
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

	for range b.N {
		t := engine.GetTask()
		t.ID = "bench_task"

		if err := engine.Submit(t); err != nil {
			b.Fatalf("engine.Submit failed during benchmark latency test: %v", err)
		}

		res := <-engine.Results()
		engine.RecycleResult(res)
	}

	b.StopTimer()
	engine.Stop()
}
