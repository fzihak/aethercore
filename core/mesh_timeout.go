package core

import (
	"context"
	"sync"
	"time"
)

// EphemeralStage describes which lifecycle phase of a distributed task was
// recorded. Stages flow: Created → Propagated → (Completed | Timeout | Failed).
type EphemeralStage string

const (
	StageCreated    EphemeralStage = "created"
	StagePropagated EphemeralStage = "propagated"
	StageCompleted  EphemeralStage = "completed"
	StageTimeout    EphemeralStage = "timeout"
	StageFailed     EphemeralStage = "failed"
)

// EphemeralEvent records one lifecycle transition of a distributed task.
type EphemeralEvent struct {
	TaskID    string
	NodeID    string
	Stage     EphemeralStage
	Timestamp time.Time
	Detail    string // optional human-readable context (e.g., error message)
}

// EphemeralLog is a thread-safe, in-memory audit log of distributed task
// lifecycle events. It supports per-task queries and TTL-based compaction.
type EphemeralLog struct {
	mu     sync.RWMutex
	events map[string][]EphemeralEvent // keyed by TaskID
}

// NewEphemeralLog allocates an empty EphemeralLog.
func NewEphemeralLog() *EphemeralLog {
	return &EphemeralLog{events: make(map[string][]EphemeralEvent)}
}

// Record appends an event to the log.
func (l *EphemeralLog) Record(ev EphemeralEvent) { //nolint:gocritic // EphemeralEvent is 88 bytes but value semantics are intentional for immutability
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	l.mu.Lock()
	l.events[ev.TaskID] = append(l.events[ev.TaskID], ev)
	l.mu.Unlock()

	WithComponent("ephemeral.log").Info("task_event",
		"task_id", ev.TaskID,
		"node_id", ev.NodeID,
		"stage", string(ev.Stage),
	)
}

// Events returns a snapshot of all recorded events for the given task ID.
// Returns nil if the task has no recorded events.
func (l *EphemeralLog) Events(taskID string) []EphemeralEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()
	src := l.events[taskID]
	if len(src) == 0 {
		return nil
	}
	out := make([]EphemeralEvent, len(src))
	copy(out, src)
	return out
}

// Purge removes all events for the given task ID.
func (l *EphemeralLog) Purge(taskID string) {
	l.mu.Lock()
	delete(l.events, taskID)
	l.mu.Unlock()
}

// PurgeExpired removes all events for tasks whose last recorded event is older
// than maxAge. This prevents unbounded memory growth for long-running nodes.
func (l *EphemeralLog) PurgeExpired(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	l.mu.Lock()
	defer l.mu.Unlock()
	removed := 0
	for taskID, evs := range l.events {
		if len(evs) == 0 {
			delete(l.events, taskID)
			removed++
			continue
		}
		last := evs[len(evs)-1]
		if last.Timestamp.Before(cutoff) {
			delete(l.events, taskID)
			removed++
		}
	}
	return removed
}

// TaskCount returns the number of distinct task IDs currently in the log.
func (l *EphemeralLog) TaskCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.events)
}

// WithTaskDeadline extracts a deadline from a PropagatedTask and applies it
// to the provided context. If DeadlineUnixNs is zero or already past, the
// context is returned with an immediate cancellation.
//
// The caller is responsible for calling the returned CancelFunc.
func WithTaskDeadline(ctx context.Context, task *PropagatedTask) (context.Context, context.CancelFunc) {
	if task.DeadlineUnixNs <= 0 {
		// No deadline — return a plain cancel-only context.
		return context.WithCancel(ctx) // #nosec G118 -- cancel func is returned to caller per function contract
	}
	deadline := time.Unix(0, task.DeadlineUnixNs)
	return context.WithDeadline(ctx, deadline) // #nosec G118 -- cancel func is returned to caller per function contract
}

// SetTaskDeadline stamps a PropagatedTask with an absolute deadline derived
// from the current time plus duration d. d ≤ 0 clears any existing deadline.
func SetTaskDeadline(task *PropagatedTask, d time.Duration) {
	if d <= 0 {
		task.DeadlineUnixNs = 0
		return
	}
	task.DeadlineUnixNs = time.Now().Add(d).UnixNano()
}

// TaskDeadlineRemaining returns how much time is left before a task's deadline.
// Returns (0, false) if no deadline is set or the deadline has already passed.
func TaskDeadlineRemaining(task *PropagatedTask) (time.Duration, bool) {
	if task.DeadlineUnixNs <= 0 {
		return 0, false
	}
	remaining := time.Until(time.Unix(0, task.DeadlineUnixNs))
	if remaining <= 0 {
		return 0, false
	}
	return remaining, true
}
