package auditlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// Sentinel errors returned by [WorkflowReport.Validate]. Consumers can match
// on these with [errors.Is] to distinguish validation failure modes without
// parsing error text.
var (
	// ErrEventCountMismatch indicates the report's EventCount field does not
	// match the length of its Events slice.
	ErrEventCountMismatch = errors.New("event_count does not match len(events)")
	// ErrStepCountMismatch indicates the report's StepCount field does not
	// match the length of its Steps slice.
	ErrStepCountMismatch = errors.New("step_count does not match len(steps)")
	// ErrStatusDrift indicates a step's stored Status disagrees with the
	// status implied by its Error pointer (see [StepInfo.DeriveStatus]).
	ErrStatusDrift = errors.New("step status does not match derived status")
)

// WorkflowReport is a consolidated, machine-readable snapshot of the audit log.
type WorkflowReport struct {
	Version                string    `json:"version"`
	WorkflowID             string    `json:"workflow_id"`
	RunID                  string    `json:"run_id,omitempty"`
	ExportedAt             time.Time `json:"exported_at"`
	EventCount             int       `json:"event_count"`
	StepCount              int       `json:"step_count"`
	SucceededCount         int       `json:"succeeded_count"`
	FailedCount            int       `json:"failed_count"`
	SkippedCount           int       `json:"skipped_count"`
	CanceledCount          int       `json:"canceled_count"`
	PendingCount           int       `json:"pending_count"`
	RunningCount           int       `json:"running_count"`
	TotalDurationMs        float64   `json:"total_duration_ms"`
	WorkflowSucceeded      bool      `json:"workflow_succeeded"`
	DroppedEventCount      int64     `json:"dropped_event_count"`
	PeakConcurrency        int       `json:"peak_concurrency,omitempty"`
	CriticalPathDurationMs float64   `json:"critical_path_duration_ms,omitempty"`
	FailureReason          string    `json:"failure_reason,omitempty"`
	// Reconstructed is true when the report was built by ReplayEvents from a
	// flat event stream rather than from live workflow hooks.
	Reconstructed bool       `json:"reconstructed,omitempty"`
	Events        []Event    `json:"events,omitempty"`
	Steps         []StepInfo `json:"steps"`
}

// Validate checks internal consistency of the report: denormalized count
// fields must match the actual slice lengths, and every step's Status must
// match its DeriveStatus. Returns nil if consistent.
//
// The status drift check catches the case where a step's stored Status field
// disagrees with what its Error pointer implies — e.g., Status=Pending with a
// non-nil Error (which DeriveStatus would map to Failed).
func (r WorkflowReport) Validate() error {
	if r.EventCount != len(r.Events) {
		return fmt.Errorf("%w: got %d, want %d", ErrEventCountMismatch, r.EventCount, len(r.Events))
	}

	if r.StepCount != len(r.Steps) {
		return fmt.Errorf("%w: got %d, want %d", ErrStepCountMismatch, r.StepCount, len(r.Steps))
	}

	for _, step := range r.Steps {
		derived := step.DeriveStatus()
		if step.Status != derived {
			return fmt.Errorf("%w: step %q has status %q but derived status is %q",
				ErrStatusDrift, step.Name, step.Status, derived)
		}
	}

	return nil
}

// StepByName returns the first StepInfo matching the given exact name.
// Returns nil if no step matches.
func (r WorkflowReport) StepByName(name string) *StepInfo {
	for i := range r.Steps {
		if r.Steps[i].Name == name {
			return &r.Steps[i]
		}
	}

	return nil
}

// EventsByStep returns all events for the given step name.
func (r WorkflowReport) EventsByStep(stepName string) []Event {
	var result []Event

	for _, e := range r.Events {
		if e.Name == stepName {
			result = append(result, e)
		}
	}

	return result
}

// EventsByType returns all events matching the given event type.
func (r WorkflowReport) EventsByType(t EventType) []Event {
	var result []Event

	for _, e := range r.Events {
		if e.EventType == t {
			result = append(result, e)
		}
	}

	return result
}

// FailedSteps returns all steps with an error status (failed or canceled).
func (r WorkflowReport) FailedSteps() []StepInfo {
	var result []StepInfo

	for _, s := range r.Steps {
		if s.Status.IsError() {
			result = append(result, s)
		}
	}

	return result
}

// SucceededSteps returns all steps that succeeded.
func (r WorkflowReport) SucceededSteps() []StepInfo {
	var result []StepInfo

	for _, s := range r.Steps {
		if s.Status == StepStatusSucceeded {
			result = append(result, s)
		}
	}

	return result
}

// SkippedSteps returns all steps that were skipped.
func (r WorkflowReport) SkippedSteps() []StepInfo {
	var result []StepInfo

	for _, s := range r.Steps {
		if s.Status == StepStatusSkipped {
			result = append(result, s)
		}
	}

	return result
}

// RetriedSteps returns all steps that had more than one attempt.
func (r WorkflowReport) RetriedSteps() []StepInfo {
	var result []StepInfo

	for _, s := range r.Steps {
		if s.AttemptCount > 1 {
			result = append(result, s)
		}
	}

	return result
}

// WriteJSON writes the report as indented JSON to the writer.
func (r WorkflowReport) WriteJSON(writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(r)
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}

	return nil
}

// WriteNDJSON writes the report's events as newline-delimited JSON.
// Each line is a single Event object. This is the inverse of ReadEvents.
func (r WorkflowReport) WriteNDJSON(writer io.Writer) error {
	return writeEventsNDJSON(writer, r.Events)
}
