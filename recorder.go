package auditlog

import (
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	flow "github.com/Azure/go-workflow"
)

const (
	// microsPerMs converts microseconds to milliseconds.
	microsPerMs = 1000.0
	// initialEventCapacity is the starting capacity for the events slice.
	initialEventCapacity = 256
)

// stepKey identifies a step by its pointer. Since go-workflow requires Steper
// to be a comparable type (typically *struct), the pointer itself is a stable
// identity throughout the workflow lifecycle.
type stepKey = flow.Steper

// attemptTracker tracks per-step attempt state during execution.
type attemptTracker struct {
	startTime time.Time
}

// stepRecord is the internal mutable state for a single step.
type stepRecord struct {
	StepRef

	stepID int

	attemptCount int
	startedAt    *time.Time
	finishedAt   *time.Time
	durationMs   *float64
	attemptErr   *string

	// pendingAttempt tracks the in-flight attempt's start time.
	pendingAttempts []attemptTracker

	// status is set by Snapshot from the workflow's final state.
	status StepStatus

	// Snapshot-enriched fields.
	maxAttempts  int
	hasRetry     bool
	hasTimeout   bool
	dependencies []StepRef
}

// Recorder captures workflow execution events in-memory with minimal overhead.
//
// # Locking Protocol
//
// All mutable state is protected by a single sync.RWMutex (mu):
//
//	Write path: mu.Lock() — recordBeforeStep, recordAfterStep, snapshot
//	Read path:  mu.RLock() — BuildReport, Events, EventsCount
//
// The onEvent callback is always called outside the lock to prevent user code
// from blocking or deadlocking the recorder.
type Recorder struct {
	mu     sync.RWMutex
	events []Event
	steps  map[stepKey]*stepRecord

	sequence    *atomic.Int64
	workflowID  string
	runID       string
	onEvent     func(Event)
	stepCounter int

	maxEvents     int
	droppedEvents atomic.Int64
}

// NewRecorder creates a new event recorder.
//
// workflowID identifies the workflow (stable across runs); runID identifies a
// single execution and is stamped on every captured Event so all observations
// from one run can be correlated. Pass a non-empty runID (e.g. a trace ID) to
// integrate with external observability systems.
func NewRecorder(workflowID, runID string, onEvent func(Event)) *Recorder {
	return &Recorder{
		mu:         sync.RWMutex{},
		events:     make([]Event, 0, initialEventCapacity),
		steps:      make(map[stepKey]*stepRecord),
		sequence:   newSequenceCounter(),
		workflowID: workflowID,
		runID:      runID,
		onEvent:    onEvent,
	}
}

// nextStepIDLocked returns the next 1-based step identifier. Caller must
// hold r.mu.
func (r *Recorder) nextStepIDLocked() int {
	r.stepCounter++

	return r.stepCounter
}

func newSequenceCounter() *atomic.Int64 {
	var counter atomic.Int64

	return &counter
}

func (r *Recorder) nextSequence() int {
	return int(r.sequence.Add(1))
}

// recordBeforeStep is called from the BeforeStep callback (per attempt).
func (r *Recorder) recordBeforeStep(step flow.Steper) {
	name := flow.String(step)
	now := time.Now()
	seq := r.nextSequence()

	r.mu.Lock()

	rec := r.getOrCreateStepLocked(step, name, now)
	rec.attemptCount++

	evt := Event{
		RunID:     r.runID,
		Sequence:  seq,
		Timestamp: now,
		EventType: EventTypeAttemptStart,
		Phase:     PhaseBefore,
		StepRef:   StepRef{Name: name, StepType: rec.StepType},
		Attempt:   rec.attemptCount,
	}
	r.appendEventLocked(evt)

	// Track this attempt's start time for duration calculation.
	rec.pendingAttempts = append(rec.pendingAttempts, attemptTracker{startTime: now})

	r.mu.Unlock()

	if r.onEvent != nil {
		r.onEvent(evt)
	}
}

// recordAfterStep is called from the AfterStep callback (per attempt).
func (r *Recorder) recordAfterStep(step flow.Steper, err error) {
	name := flow.String(step)
	now := time.Now()
	seq := r.nextSequence()
	errStr := errorToStringPtr(err)

	r.mu.Lock()

	rec, ok := r.steps[step]
	if !ok {
		rec = r.getOrCreateStepLocked(step, name, now)
	}

	var durationMs *float64

	// Pop the most recent pending attempt (LIFO).
	if len(rec.pendingAttempts) > 0 {
		idx := len(rec.pendingAttempts) - 1
		d := float64(now.Sub(rec.pendingAttempts[idx].startTime).Microseconds()) / microsPerMs
		durationMs = &d
		rec.pendingAttempts = rec.pendingAttempts[:idx]
	}

	// Record the finish time and error from the last attempt. We always
	// overwrite — a nil errStr (success) clears any stale error from a
	// previous failed attempt, so the step-level Error field reflects the
	// final outcome only. The per-attempt error history is preserved in
	// the Event stream (each attempt_end carries its own Error pointer).
	rec.finishedAt = &now
	rec.attemptErr = errStr

	// Only overwrite the duration if we measured one; otherwise keep the
	// previous attempt's duration so a stray AfterStep without a matching
	// BeforeStep doesn't clobber a valid value.
	if durationMs != nil {
		rec.durationMs = durationMs
	}

	status := fromErrorToStatus(err)

	evt := Event{
		RunID:      r.runID,
		Sequence:   seq,
		Timestamp:  now,
		EventType:  EventTypeAttemptEnd,
		Phase:      PhaseAfter,
		StepRef:    StepRef{Name: name, StepType: rec.StepType},
		Attempt:    rec.attemptCount,
		DurationMs: durationMs,
		Error:      errStr,
		Status:     status,
	}
	r.appendEventLocked(evt)

	r.mu.Unlock()

	if r.onEvent != nil {
		r.onEvent(evt)
	}
}

// getOrCreateStepLocked returns the step record, creating it if needed.
// Caller must hold r.mu.
func (r *Recorder) getOrCreateStepLocked(step flow.Steper, name string, now time.Time) *stepRecord {
	if rec, ok := r.steps[step]; ok {
		return rec
	}

	rec := &stepRecord{
		StepRef:   StepRef{Name: name, StepType: stepTypeName(step)},
		stepID:    r.nextStepIDLocked(),
		startedAt: &now,
		status:    StepStatusRunning,
	}
	r.steps[step] = rec

	return rec
}

// appendEventLocked appends an event, respecting the MaxEvents cap.
// Caller must hold r.mu.
func (r *Recorder) appendEventLocked(evt Event) {
	if r.maxEvents > 0 && len(r.events) >= r.maxEvents {
		r.droppedEvents.Add(1)

		return
	}

	r.events = append(r.events, evt)
}

// DroppedEventCount returns the number of events dropped due to MaxEvents cap.
func (r *Recorder) DroppedEventCount() int64 {
	return r.droppedEvents.Load()
}

// Events returns a defensive copy of all captured events.
func (r *Recorder) Events() []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return append([]Event(nil), r.events...)
}

// EventsCount returns the number of captured events without copying the slice.
func (r *Recorder) EventsCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.events)
}

// RunID returns the run identifier stamped on every captured event. It is safe
// to call concurrently.
func (r *Recorder) RunID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.runID
}

// errorToStringPtr converts an error to a heap-allocated string pointer.
// Returns nil when err is nil so we don't emit empty error fields in events.
func errorToStringPtr(err error) *string {
	if err == nil {
		return nil
	}

	msg := err.Error()

	return &msg
}

// fromErrorToStatus maps an error to a StepStatus using go-workflow's semantics.
// nil → succeeded, wrapped skip/cancel → corresponding status, else → failed.
func fromErrorToStatus(err error) StepStatus {
	return fromFlowStatus(string(flow.StatusFromError(err)))
}

// stepTypeName returns a human-readable type name for a step using reflection.
// For known flow types (Function, NoOpStep, MockStep), returns the base name.
// For custom structs, returns the struct type name.
func stepTypeName(step flow.Steper) string {
	if step == nil {
		return ""
	}

	t := reflect.TypeOf(step)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	name := t.Name()
	if name != "" {
		return name
	}

	return t.String()
}
