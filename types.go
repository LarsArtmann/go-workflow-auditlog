package auditlog

import (
	"cmp"
	"slices"
)

// SchemaVersion is the current report schema version.
const SchemaVersion = "0.1.0"

// EventType categorizes audit log events.
//
// Every event is one of two types, mirroring the two go-workflow callbacks:
// AttemptStart (from BeforeStep) and AttemptEnd (from AfterStep). EventType is
// intentionally redundant with Phase — an AttemptStart always carries
// PhaseBefore, an AttemptEnd always carries PhaseAfter. Both fields are kept so
// consumers can filter by either axis (event kind or lifecycle position)
// without cross-referencing.
type EventType string

const (
	// EventTypeAttemptStart fires when a step attempt begins (each retry try).
	EventTypeAttemptStart EventType = "attempt_start"
	// EventTypeAttemptEnd fires when a step attempt finishes (each retry try).
	EventTypeAttemptEnd EventType = "attempt_end"
)

// eventTypeMeta holds display metadata for each [EventType] value.
// Centralizing the label/color here keeps the per-event-type presentation
// in one place.
//
//nolint:gochecknoglobals // Lookup table, treated as immutable after init.
var eventTypeMeta = map[EventType]struct {
	Label string
	Color string
}{
	EventTypeAttemptStart: {Label: "Attempt Start", Color: "var(--success)"},
	EventTypeAttemptEnd:   {Label: "Attempt End", Color: "var(--warning)"},
}

// Label returns the human-readable display label for this event type.
func (e EventType) Label() string {
	if m, ok := eventTypeMeta[e]; ok {
		return m.Label
	}

	return ""
}

// Color returns the CSS color token for this event type, used in HTML visualizations.
func (e EventType) Color() string {
	if m, ok := eventTypeMeta[e]; ok {
		return m.Color
	}

	return ""
}

// Phase indicates whether an event is the start or end of an operation.
//
// It is deliberately redundant with EventType: AttemptStart ↔ PhaseBefore and
// AttemptEnd ↔ PhaseAfter. The duplication is retained in the JSON output so
// that consumers can filter on lifecycle position ("before"/"after") without
// knowing the event-type vocabulary, and vice versa.
type Phase string

const (
	PhaseBefore Phase = "before"
	PhaseAfter  Phase = "after"
)

// StepStatus mirrors [flow.StepStatus] as a stable string enum for JSON export.
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusSucceeded StepStatus = "succeeded"
	StepStatusFailed    StepStatus = "failed"
	StepStatusCanceled  StepStatus = "canceled"
	StepStatusSkipped   StepStatus = "skipped"
)

// stepStatusMeta holds display metadata for each [StepStatus] value.
// Centralizing the label/icon here keeps the per-status presentation in one
// place and makes new statuses a one-line addition.
//
//nolint:gochecknoglobals // Lookup table, treated as immutable after init.
var stepStatusMeta = map[StepStatus]struct {
	Label string
	Icon  string
}{
	StepStatusPending:   {Label: "Pending", Icon: "\u26AA"},
	StepStatusRunning:   {Label: "Running", Icon: "\U0001F7E1"},
	StepStatusSucceeded: {Label: "Succeeded", Icon: "\U0001F7E2"},
	StepStatusFailed:    {Label: "Failed", Icon: "\U0001F534"},
	StepStatusCanceled:  {Label: "Canceled", Icon: "\U0001F6AB"},
	StepStatusSkipped:   {Label: "Skipped", Icon: "\u23ED\uFE0F"},
}

// String returns the step status name.
func (s StepStatus) String() string { return string(s) }

// Label returns the human-readable display label for this step status.
func (s StepStatus) Label() string {
	if m, ok := stepStatusMeta[s]; ok {
		return m.Label
	}

	return ""
}

// IsTerminal returns true if the step has reached a terminal state
// (succeeded, failed, canceled, or skipped).
func (s StepStatus) IsTerminal() bool {
	switch s {
	case StepStatusSucceeded, StepStatusFailed, StepStatusCanceled, StepStatusSkipped:
		return true
	default:
		return false
	}
}

// IsError returns true if the step failed or was canceled.
func (s StepStatus) IsError() bool {
	return s == StepStatusFailed || s == StepStatusCanceled
}

// Icon returns a display emoji for this step status.
func (s StepStatus) Icon() string {
	if m, ok := stepStatusMeta[s]; ok {
		return m.Icon
	}

	return ""
}

// StepRef identifies a step within a workflow.
// Embedded in Event and StepInfo for JSON flattening.
type StepRef struct {
	Name     string `json:"step_name"`
	StepType string `json:"step_type,omitempty"`
}

// sortByName sorts a slice of StepRef in place by Name, in ascending order.
// Used to give Dependencies and Dependents deterministic output across runs.
func sortByName(refs []StepRef) {
	slices.SortFunc(refs, func(a, b StepRef) int {
		return cmp.Compare(a.Name, b.Name)
	})
}

// sortStepsByName sorts a slice of StepInfo in place by Name, in ascending
// order. Used to produce deterministic step ordering across runs.
func sortStepsByName(steps []StepInfo) {
	slices.SortFunc(steps, func(a, b StepInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})
}

// fromFlowStatus converts a [flow.StepStatus] string to our StepStatus enum.
// go-workflow uses capitalized strings ("Succeeded", "Failed", etc.) while we
// use lowercase for JSON snake_case consistency.
func fromFlowStatus(s string) StepStatus {
	switch s {
	case "Running":
		return StepStatusRunning
	case "Failed":
		return StepStatusFailed
	case "Succeeded":
		return StepStatusSucceeded
	case "Canceled":
		return StepStatusCanceled
	case "Skipped":
		return StepStatusSkipped
	default: // "" (Pending) or unknown
		return StepStatusPending
	}
}
