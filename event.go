package auditlog

import "time"

// Event is a single, timestamped observation from a workflow step execution.
type Event struct {
	StepRef

	RunID         RunID         `json:"run_id,omitempty"`
	Sequence      int           `json:"sequence"`
	Timestamp     time.Time     `json:"timestamp"`
	EventType     EventType     `json:"event_type"`
	Phase         Phase         `json:"phase"`
	Attempt       int           `json:"attempt,omitempty"`
	DurationMs    *float64      `json:"duration_ms,omitempty"`
	Error         *string       `json:"error,omitempty"`
	Status        StepStatus    `json:"status,omitempty"`
	FailureReason FailureReason `json:"failure_reason,omitempty"`
}

// IsAttemptStart returns true if the event is an attempt-start event.
func (e Event) IsAttemptStart() bool { return e.EventType == EventTypeAttemptStart }

// IsAttemptEnd returns true if the event is an attempt-end event.
func (e Event) IsAttemptEnd() bool { return e.EventType == EventTypeAttemptEnd }

// IsBefore returns true if the event is the start (before) phase of an operation.
func (e Event) IsBefore() bool { return e.Phase == PhaseBefore }

// IsAfter returns true if the event is the end (after) phase of an operation.
func (e Event) IsAfter() bool { return e.Phase == PhaseAfter }

// HasError returns true if the event recorded an error.
func (e Event) HasError() bool { return e.Error != nil }

// IsTimeout returns true if the event's FailureReason is FailureReasonTimeout.
func (e Event) IsTimeout() bool { return e.FailureReason == FailureReasonTimeout }

// IsCanceled returns true if the event's FailureReason is FailureReasonCanceled.
func (e Event) IsCanceled() bool { return e.FailureReason == FailureReasonCanceled }

// IsPanic returns true if the event's FailureReason is FailureReasonPanic.
func (e Event) IsPanic() bool { return e.FailureReason == FailureReasonPanic }

// IsDependencyFailure returns true if the event's FailureReason is
// FailureReasonDependency (the step failed because an upstream did, not
// because its own Do() returned an error).
func (e Event) IsDependencyFailure() bool {
	return e.FailureReason == FailureReasonDependency
}

// IsUserError returns true if the event's FailureReason is FailureReasonUserError
// (the step's own Do() returned a non-nil error).
func (e Event) IsUserError() bool { return e.FailureReason == FailureReasonUserError }

// Duration returns the event duration in milliseconds, or 0 if unavailable.
func (e Event) Duration() float64 {
	if e.DurationMs == nil {
		return 0
	}

	return *e.DurationMs
}
