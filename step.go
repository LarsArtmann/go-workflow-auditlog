package auditlog

import "time"

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
