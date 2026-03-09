// Package scheduler implements silent background task scheduling with
// cron-like syntax native to AetherCore, enabling proactive agent behavior
// without a user prompt.
//
// The Scheduler runs as a Layer 1 Capability Module. It maintains a set of
// named jobs, each with a cron expression and a goal string. On each tick
// (every 30 seconds), the scheduler evaluates which jobs should fire and
// dispatches them as sdk.ModuleTasks to a user-supplied TaskHandler.
//
// Thread Safety: All exported methods on Scheduler are safe for concurrent use.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fzihak/aethercore/sdk"
)

// Sentinel errors for scheduler operations.
var (
	ErrJobExists   = errors.New("scheduler: job already exists")
	ErrJobNotFound = errors.New("scheduler: job not found")
	ErrNoHandler   = errors.New("scheduler: no task handler registered")
)

// tickInterval controls how often the scheduler checks for due jobs.
// Set to 30 seconds to balance responsiveness with CPU usage — the cron
// resolution is 1 minute so checking twice per minute guarantees we never
// miss a firing.
const tickInterval = 30 * time.Second

// TaskDispatcher is called by the Scheduler when a cron job fires.
// Implementations should forward the task to the AetherCore engine or
// module registry for execution.
type TaskDispatcher func(ctx context.Context, task *sdk.ModuleTask)

// Job represents a single scheduled background task.
type Job struct {
	Name     string   `json:"name"`
	Schedule CronExpr `json:"-"`
	CronRaw  string   `json:"cron"`
	Goal     string   `json:"goal"`
	Enabled  bool     `json:"enabled"`

	// lastFired tracks the last minute this job was dispatched to prevent
	// double-firing within the same minute window.
	lastFired time.Time
}

// Scheduler orchestrates background cron-based task execution.
// It implements the sdk.Module interface and can be loaded into the
// AetherCore kernel's ModuleRegistry.
type Scheduler struct {
	jobs       map[string]*Job
	mu         sync.RWMutex
	dispatcher TaskDispatcher
	quit       chan struct{}
	stopOnce   sync.Once
	log        *slog.Logger
	running    bool
	nodeID     string
}

// New creates a Scheduler with a logger and node identity.
func New(nodeID string) *Scheduler {
	opts := &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
			}
			if a.Key == slog.MessageKey {
				a.Key = "msg"
			}
			return a
		},
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts)).With(
		slog.String("service.name", "aethercore"),
		slog.String("component", "scheduler"),
	)

	return &Scheduler{
		jobs:   make(map[string]*Job),
		quit:   make(chan struct{}),
		log:    logger,
		nodeID: nodeID,
	}
}

// ── sdk.Module interface ────────────────────────────────────────────────

// Manifest returns the module's static metadata.
func (s *Scheduler) Manifest() sdk.ModuleManifest {
	return sdk.ModuleManifest{
		Name:             "scheduler",
		Description:      "Cron-like background task scheduler for proactive agent behavior",
		Version:          "1.0.0",
		Author:           "AetherCore",
		Capabilities:     []sdk.Capability{sdk.CapState},
		MaxTaskRuntimeMs: 10_000,
		MemoryLimitMB:    32,
	}
}

// OnStart initializes the scheduler when loaded into the kernel.
func (s *Scheduler) OnStart(_ context.Context, mc *sdk.ModuleContext) error {
	s.log.Info("scheduler_module_started", slog.String("node_id", s.nodeID))
	return nil
}

// OnStop gracefully shuts down the tick loop.
func (s *Scheduler) OnStop(_ context.Context) error {
	s.Stop()
	return nil
}

// HandleTask allows external callers to manage the scheduler via task dispatch.
// Supported inputs (JSON):
//
//	{"action":"list"}
//	{"action":"add","name":"daily-check","cron":"0 9 * * *","goal":"Run health check"}
//	{"action":"remove","name":"daily-check"}
//	{"action":"enable","name":"daily-check"}
//	{"action":"disable","name":"daily-check"}
func (s *Scheduler) HandleTask(_ context.Context, task *sdk.ModuleTask) (*sdk.ModuleResult, error) {
	// For V1 the scheduler is managed programmatically; this is a stub for
	// future JSON-based management via the LLM tool loop.
	return &sdk.ModuleResult{
		TaskID: task.ID,
		Output: fmt.Sprintf("scheduler: %d active jobs", s.Len()),
	}, nil
}

// ── Scheduler API ───────────────────────────────────────────────────────

// SetDispatcher registers the callback invoked when a cron job fires.
// Must be called before Start.
func (s *Scheduler) SetDispatcher(fn TaskDispatcher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher = fn
}

// AddJob registers a new recurring job. Returns ErrJobExists if a job
// with the same name is already registered.
func (s *Scheduler) AddJob(name, cronExpr, goal string) error {
	parsed, err := ParseCron(cronExpr)
	if err != nil {
		return fmt.Errorf("scheduler: add %q: %w", name, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[name]; exists {
		return fmt.Errorf("%w: %s", ErrJobExists, name)
	}

	s.jobs[name] = &Job{
		Name:     name,
		Schedule: parsed,
		CronRaw:  cronExpr,
		Goal:     goal,
		Enabled:  true,
	}

	s.log.Info("job_added",
		slog.String("name", name),
		slog.String("cron", cronExpr),
		slog.String("goal", goal),
	)
	return nil
}

// RemoveJob deletes a job by name. Returns ErrJobNotFound if absent.
func (s *Scheduler) RemoveJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[name]; !exists {
		return fmt.Errorf("%w: %s", ErrJobNotFound, name)
	}
	delete(s.jobs, name)

	s.log.Info("job_removed", slog.String("name", name))
	return nil
}

// EnableJob activates a previously disabled job.
func (s *Scheduler) EnableJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrJobNotFound, name)
	}
	job.Enabled = true
	return nil
}

// DisableJob deactivates a job without removing it.
func (s *Scheduler) DisableJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrJobNotFound, name)
	}
	job.Enabled = false
	return nil
}

// Jobs returns a snapshot of all registered jobs.
func (s *Scheduler) Jobs() []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, *j)
	}
	return out
}

// Len returns the number of registered jobs.
func (s *Scheduler) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.jobs)
}

// Start begins the background tick loop. Cancel via Stop() or context.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.log.Info("scheduler_tick_loop_started", slog.Duration("interval", tickInterval))

	go s.tickLoop(ctx)
}

// Stop shuts down the tick loop. Safe to call multiple times.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.quit)
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		s.log.Info("scheduler_tick_loop_stopped")
	})
}

// tickLoop is the main scheduler goroutine. It wakes up every tickInterval,
// checks which jobs are due, and dispatches them.
func (s *Scheduler) tickLoop(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.quit:
			return
		case <-ctx.Done():
			s.Stop()
			return
		case now := <-ticker.C:
			s.evaluateAndDispatch(ctx, now)
		}
	}
}

// evaluateAndDispatch checks every enabled job against the current time
// and dispatches any that match.
func (s *Scheduler) evaluateAndDispatch(ctx context.Context, now time.Time) {
	// Truncate to minute boundary for cron comparison
	nowMinute := now.Truncate(time.Minute)

	s.mu.Lock()
	dispatcher := s.dispatcher
	var dueJobs []*Job
	for _, job := range s.jobs {
		if !job.Enabled {
			continue
		}
		// Skip if we already fired this job during this minute
		if job.lastFired.Truncate(time.Minute).Equal(nowMinute) {
			continue
		}
		if job.Schedule.Matches(nowMinute) {
			job.lastFired = now
			dueJobs = append(dueJobs, job)
		}
	}
	s.mu.Unlock()

	if dispatcher == nil || len(dueJobs) == 0 {
		return
	}

	for _, job := range dueJobs {
		task := &sdk.ModuleTask{
			ID:    fmt.Sprintf("sched-%s-%d", job.Name, now.UnixNano()),
			Input: job.Goal,
			Metadata: map[string]string{
				"source":    "scheduler",
				"job_name":  job.Name,
				"cron_expr": job.CronRaw,
				"scheduled": nowMinute.Format(time.RFC3339),
			},
		}

		s.log.Info("job_dispatched",
			slog.String("job", job.Name),
			slog.String("task_id", task.ID),
			slog.String("goal", job.Goal),
		)

		// Dispatch in a goroutine so slow handlers don't block the tick loop
		go dispatcher(ctx, task)
	}
}
