package auditlog

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/larsartmann/go-output"
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
	// ErrCountMismatch indicates a denormalized status-count field
	// (SucceededCount, FailedCount, etc.) disagrees with the actual count
	// derived from the Steps slice.
	ErrCountMismatch = errors.New("status count does not match steps")

	// ErrRenderFailed wraps errors encountered while rendering or marshaling a
	// report for output (JSON encoding, diagram rendering, HTML generation,
	// table/tree rendering). Classified as Infrastructure — these failures are
	// not retryable (programming error or resource exhaustion).
	// Consumers can match on it with [errors.Is].
	ErrRenderFailed = errors.New("render failed")
)

// WorkflowReport is a consolidated, machine-readable snapshot of the audit log.
type WorkflowReport struct {
	Version                string    `json:"version"`
	WorkflowID             string    `json:"workflow_id"`
	RunID                  RunID     `json:"run_id,omitempty"`
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
	WallClockDurationMs    float64   `json:"wall_clock_duration_ms"`
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

	err := validateStatusCounts(r)

	return err
}

// validateStatusCounts verifies the denormalized status-count fields match
// the actual counts derived from the Steps slice.
func validateStatusCounts(r WorkflowReport) error {
	want := map[StepStatus]int{
		StepStatusSucceeded: r.SucceededCount,
		StepStatusFailed:    r.FailedCount,
		StepStatusSkipped:   r.SkippedCount,
		StepStatusCanceled:  r.CanceledCount,
		StepStatusPending:   r.PendingCount,
		StepStatusRunning:   r.RunningCount,
	}

	got := make(map[StepStatus]int)

	for _, step := range r.Steps {
		got[step.Status]++
	}

	for status, expected := range want {
		if got[status] != expected {
			return fmt.Errorf("%w: %s expected %d, got %d",
				ErrCountMismatch, status, expected, got[status])
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
	encoder := jsontext.NewEncoder(writer, jsontext.WithIndent("  "))

	err := json.MarshalEncode(encoder, r,
		json.Deterministic(true),
		jsontext.EscapeForHTML(true),
		jsontext.EscapeForJS(true),
	)
	if err != nil {
		return fmt.Errorf("%w: encode report: %w", ErrRenderFailed, err)
	}

	return nil
}

// WriteNDJSON writes the report's events as newline-delimited JSON.
// Each line is a single Event object. This is the inverse of ReadEvents.
func (r WorkflowReport) WriteNDJSON(writer io.Writer) error {
	return writeEventsNDJSON(writer, r.Events)
}

// Duration returns the total wall-clock duration spanned by all events,
// from the earliest to the latest timestamp. This is different from
// TotalDurationMs (which sums individual step durations and may overcount
// when steps run in parallel). The same value is available as the
// WallClockDurationMs JSON field.
func (r WorkflowReport) Duration() time.Duration {
	return time.Duration(computeWallClockDurationMs(r.Events) * float64(time.Millisecond))
}

// computeWallClockDurationMs returns the elapsed wall-clock time in
// milliseconds from the earliest to the latest event timestamp. Unlike
// TotalDurationMs (which sums per-step durations and overcounts for parallel
// workflows), this reflects the actual time the workflow occupied.
func computeWallClockDurationMs(events []Event) float64 {
	if len(events) == 0 {
		return 0
	}

	earliest := events[0].Timestamp
	latest := events[0].Timestamp

	for _, evt := range events {
		if evt.Timestamp.Before(earliest) {
			earliest = evt.Timestamp
		}

		if evt.Timestamp.After(latest) {
			latest = evt.Timestamp
		}
	}

	return float64(latest.Sub(earliest).Microseconds()) / microsPerMs
}

// Summary returns a human-readable one-line summary of the report.
// Uses wall-clock duration (actual elapsed time) rather than the summed
// per-step duration, and includes the failure reason when the workflow
// did not succeed.
func (r WorkflowReport) Summary() string {
	base := fmt.Sprintf("%s: %d steps (%d ok, %d failed, %d skipped) in %.1fms",
		r.WorkflowID, r.StepCount, r.SucceededCount, r.FailedCount,
		r.SkippedCount, r.WallClockDurationMs)

	if r.WorkflowSucceeded {
		return base
	}

	if r.FailureReason != "" {
		return base + " — " + r.FailureReason
	}

	return base + " — failed"
}

// --- File export methods (mirror Auditor.Export*) ---
//
// These let a caller holding a WorkflowReport — for example one rebuilt via
// ReplayEvents from an NDJSON stream — write every format to a file without
// needing an Auditor instance.

// ExportJSON writes the report as indented JSON to path.
func (r WorkflowReport) ExportJSON(path string) error {
	return writeToFile(path, r.WriteJSON)
}

// ExportNDJSON writes the report's events as NDJSON to path.
func (r WorkflowReport) ExportNDJSON(path string) error {
	return writeToFile(path, r.WriteNDJSON)
}

// ExportMermaid writes the step DAG as a Mermaid diagram to path.
func (r WorkflowReport) ExportMermaid(path string) error {
	return writeToFile(path, r.WriteMermaid)
}

// ExportPlantUML writes the step DAG as a PlantUML diagram to path.
func (r WorkflowReport) ExportPlantUML(path string) error {
	return writeToFile(path, r.WritePlantUML)
}

// ExportGraphviz writes the step DAG as a Graphviz DOT diagram to path.
func (r WorkflowReport) ExportGraphviz(path string) error {
	return writeToFile(path, r.WriteGraphviz)
}

// ExportD2 writes the step DAG as a D2 diagram to path.
func (r WorkflowReport) ExportD2(path string) error {
	return writeToFile(path, r.WriteD2)
}

// ExportTable writes the step summary table to path in the given format.
func (r WorkflowReport) ExportTable(path string, format output.Format, opts output.RenderOptions) error {
	return writeToFile(path, func(w io.Writer) error {
		return r.WriteTable(w, format, opts)
	})
}

// ExportTree writes the step DAG as an ASCII tree to path.
func (r WorkflowReport) ExportTree(path string) error {
	return writeToFile(path, r.WriteTree)
}

// ExportHTMLTree writes the step DAG as an HTML nested-list tree to path.
func (r WorkflowReport) ExportHTMLTree(path string) error {
	return writeToFile(path, r.WriteHTMLTree)
}
