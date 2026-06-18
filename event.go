package auditlog

import "time"

// Event is a single, timestamped observation from a workflow step execution.
type Event struct {
	StepRef

	RunID      string     `json:"run_id,omitempty"`
	Sequence   int        `json:"sequence"`
	Timestamp  time.Time  `json:"timestamp"`
	EventType  EventType  `json:"event_type"`
	Phase      Phase      `json:"phase"`
	Attempt    int        `json:"attempt,omitempty"`
	DurationMs *float64   `json:"duration_ms,omitempty"`
	Error      *string    `json:"error,omitempty"`
	Status     StepStatus `json:"status,omitempty"`
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

// Duration returns the event duration in milliseconds, or 0 if unavailable.
func (e Event) Duration() float64 {
	if e.DurationMs == nil {
		return 0
	}

	return *e.DurationMs
}
