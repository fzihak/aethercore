package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fzihak/aethercore/sdk"
)

// ── Scheduler lifecycle tests ───────────────────────────────────────────

func TestSchedulerManifest(t *testing.T) {
	s := New("test-node")
	m := s.Manifest()
	if m.Name != "scheduler" {
		t.Errorf("Manifest().Name = %q, want %q", m.Name, "scheduler")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Manifest().Version = %q, want %q", m.Version, "1.0.0")
	}
}

func TestSchedulerModuleInterface(t *testing.T) {
	// Verify Scheduler satisfies sdk.Module at compile time
	var _ sdk.Module = (*Scheduler)(nil)
}

func TestSchedulerOnStartOnStop(t *testing.T) {
	s := New("test-node")
	mc := sdk.NewModuleContext("scheduler")

	if err := s.OnStart(context.Background(), mc); err != nil {
		t.Fatalf("OnStart error: %v", err)
	}
	if err := s.OnStop(context.Background()); err != nil {
		t.Fatalf("OnStop error: %v", err)
	}
}

// ── Job management tests ────────────────────────────────────────────────

func TestAddJob(t *testing.T) {
	s := New("test-node")
	err := s.AddJob("health-check", "*/5 * * * *", "Run health check")
	if err != nil {
		t.Fatalf("AddJob error: %v", err)
	}
	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1", s.Len())
	}
}

func TestAddJobDuplicate(t *testing.T) {
	s := New("test-node")
	_ = s.AddJob("check", "* * * * *", "goal1")
	err := s.AddJob("check", "* * * * *", "goal2")
	if !errors.Is(err, ErrJobExists) {
		t.Errorf("expected ErrJobExists, got %v", err)
	}
}

func TestAddJobInvalidCron(t *testing.T) {
	s := New("test-node")
	err := s.AddJob("bad", "not a cron", "goal")
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestRemoveJob(t *testing.T) {
	s := New("test-node")
	_ = s.AddJob("check", "* * * * *", "goal")
	err := s.RemoveJob("check")
	if err != nil {
		t.Fatalf("RemoveJob error: %v", err)
	}
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
}

func TestRemoveJobNotFound(t *testing.T) {
	s := New("test-node")
	err := s.RemoveJob("nonexistent")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestEnableDisableJob(t *testing.T) {
	s := New("test-node")
	_ = s.AddJob("check", "* * * * *", "goal")

	if err := s.DisableJob("check"); err != nil {
		t.Fatalf("DisableJob error: %v", err)
	}
	jobs := s.Jobs()
	for _, j := range jobs {
		if j.Name == "check" && j.Enabled {
			t.Error("expected job to be disabled")
		}
	}

	if err := s.EnableJob("check"); err != nil {
		t.Fatalf("EnableJob error: %v", err)
	}
	jobs = s.Jobs()
	for _, j := range jobs {
		if j.Name == "check" && !j.Enabled {
			t.Error("expected job to be enabled")
		}
	}
}

func TestEnableDisableNotFound(t *testing.T) {
	s := New("test-node")
	if err := s.EnableJob("ghost"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
	if err := s.DisableJob("ghost"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestJobsSnapshot(t *testing.T) {
	s := New("test-node")
	_ = s.AddJob("a", "0 * * * *", "goal-a")
	_ = s.AddJob("b", "30 * * * *", "goal-b")

	jobs := s.Jobs()
	if len(jobs) != 2 {
		t.Fatalf("Jobs() len = %d, want 2", len(jobs))
	}
	// Verify it's a copy — mutation should not affect internal state
	jobs[0].Goal = "mutated"
	internal := s.Jobs()
	for _, j := range internal {
		if j.Goal == "mutated" {
			t.Error("Jobs() returned a reference to internal state, not a copy")
		}
	}
}

// ── Dispatch tests ──────────────────────────────────────────────────────

func TestEvaluateAndDispatch(t *testing.T) {
	s := New("test-node")

	var mu sync.Mutex
	var dispatched []*sdk.ModuleTask

	s.SetDispatcher(func(_ context.Context, task *sdk.ModuleTask) {
		mu.Lock()
		dispatched = append(dispatched, task)
		mu.Unlock()
	})

	// Add a job that matches at minute 0 of every hour
	_ = s.AddJob("hourly", "0 * * * *", "hourly goal")

	// Simulate a tick at exactly :00
	now := time.Date(2026, 3, 9, 14, 0, 0, 0, time.UTC)
	s.evaluateAndDispatch(context.Background(), now)

	// Give the goroutine a moment
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(dispatched) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(dispatched))
	}
	if dispatched[0].Input != "hourly goal" {
		t.Errorf("Input = %q, want %q", dispatched[0].Input, "hourly goal")
	}
	if dispatched[0].Metadata["source"] != "scheduler" {
		t.Errorf("Metadata[source] = %q, want %q", dispatched[0].Metadata["source"], "scheduler")
	}
	if dispatched[0].Metadata["job_name"] != "hourly" {
		t.Errorf("Metadata[job_name] = %q, want %q", dispatched[0].Metadata["job_name"], "hourly")
	}
}

func TestDoubleFirPrevention(t *testing.T) {
	s := New("test-node")

	var mu sync.Mutex
	count := 0
	s.SetDispatcher(func(_ context.Context, _ *sdk.ModuleTask) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	_ = s.AddJob("test", "0 * * * *", "goal")

	now := time.Date(2026, 3, 9, 14, 0, 0, 0, time.UTC)
	// Fire twice in the same minute window
	s.evaluateAndDispatch(context.Background(), now)
	s.evaluateAndDispatch(context.Background(), now.Add(15*time.Second))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 dispatch (double-fire prevention), got %d", count)
	}
}

func TestDisabledJobNotDispatched(t *testing.T) {
	s := New("test-node")

	count := 0
	s.SetDispatcher(func(_ context.Context, _ *sdk.ModuleTask) {
		count++
	})

	_ = s.AddJob("test", "* * * * *", "goal")
	_ = s.DisableJob("test")

	now := time.Date(2026, 3, 9, 14, 30, 0, 0, time.UTC)
	s.evaluateAndDispatch(context.Background(), now)

	time.Sleep(50 * time.Millisecond)
	if count != 0 {
		t.Errorf("expected 0 dispatches for disabled job, got %d", count)
	}
}

func TestNoDispatcherSafe(t *testing.T) {
	s := New("test-node")
	_ = s.AddJob("test", "* * * * *", "goal")
	// No dispatcher set — should not panic
	now := time.Date(2026, 3, 9, 14, 30, 0, 0, time.UTC)
	s.evaluateAndDispatch(context.Background(), now) // should be a no-op
}

func TestHandleTask(t *testing.T) {
	s := New("test-node")
	_ = s.AddJob("a", "* * * * *", "goal")
	_ = s.AddJob("b", "0 * * * *", "other")

	result, err := s.HandleTask(context.Background(), &sdk.ModuleTask{ID: "t1"})
	if err != nil {
		t.Fatalf("HandleTask error: %v", err)
	}
	if result.TaskID != "t1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "t1")
	}
	if result.Output != "scheduler: 2 active jobs" {
		t.Errorf("Output = %q", result.Output)
	}
}

// ── Start/Stop lifecycle ────────────────────────────────────────────────

func TestStartStop(t *testing.T) {
	s := New("test-node")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	// Double-start should be a no-op
	s.Start(ctx)

	s.Stop()
	// Double-stop should be safe
	s.Stop()
}

func TestStartContextCancel(t *testing.T) {
	s := New("test-node")
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	// Cancel should trigger Stop
	cancel()
	time.Sleep(100 * time.Millisecond)

	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()
	if running {
		t.Error("expected scheduler to be stopped after context cancel")
	}
}
