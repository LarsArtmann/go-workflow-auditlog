package auditlog

import "time"

// stepCore holds the common step-state fields shared between live capture
// (stepRecord in recorder.go) and event-stream replay (replay.go). Both paths
// accumulate the same data from different sources; this shared type ensures
// they can never diverge. The single toStepInfo method produces the public
// StepInfo with the fields available from the core alone.
type stepCore struct {
	StepRef

	attemptCount int
	startedAt    *time.Time
	finishedAt   *time.Time
	durationMs   *float64
	attemptErr   *string
	status       StepStatus
}

// toStepInfo builds a public StepInfo from the core accumulator fields.
// Callers that have additional live-only fields (stepID, maxAttempts, etc.)
// add them after calling this.
func (c stepCore) toStepInfo() StepInfo {
	return StepInfo{
		StepRef:      c.StepRef,
		Status:       c.status,
		AttemptCount: c.attemptCount,
		StartedAt:    c.startedAt,
		FinishedAt:   c.finishedAt,
		DurationMs:   c.durationMs,
		Error:        c.attemptErr,
	}
}

// StepInfo aggregates all observed data for a single workflow step.
type StepInfo struct {
	StepRef

	// StepID is a 1-based, unique identifier assigned when the step is first
	// observed. It disambiguates steps that share the same Name (which can
	// happen when two step types produce identical String() output). Stable
	// within a single report/run; not guaranteed stable across runs.
	StepID       int        `json:"step_id,omitempty"`
	Status       StepStatus `json:"status"`
	AttemptCount int        `json:"attempt_count"`
	MaxAttempts  int        `json:"max_attempts,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	DurationMs   *float64   `json:"duration_ms,omitempty"`
	Dependencies []StepRef  `json:"dependencies,omitempty"`
	Dependents   []StepRef  `json:"dependents,omitempty"`
	Error        *string    `json:"error,omitempty"`
	HasRetry     bool       `json:"has_retry"`
	HasTimeout   bool       `json:"has_timeout"`
}

// HasError returns true if the step recorded an error.
func (s StepInfo) HasError() bool { return s.Error != nil }

// Type returns the step's Go type name (e.g., "FetchStep"). This is the
// programmatic identifier of the step implementation. Provides method-style
// access for API consistency with [StepStatus.Label], [StepStatus.Icon],
// and [StepStatus.Color].
func (s StepInfo) Type() string { return s.StepType }

// Duration returns the step duration in milliseconds, or 0 if unavailable.
func (s StepInfo) Duration() float64 {
	if s.DurationMs == nil {
		return 0
	}

	return *s.DurationMs
}

// DeriveStatus computes the step status from the step's own error pointer.
// This is the canonical derivation — the stored Status field should always
// match this method so it can never drift from the underlying data.
//
// If Snapshot() has not been called yet, the status may be pending/running.
// After Snapshot(), the status reflects the workflow's final state.
func (s StepInfo) DeriveStatus() StepStatus {
	if s.Status.IsTerminal() {
		return s.Status
	}

	if s.Error != nil {
		return StepStatusFailed
	}

	return s.Status
}
