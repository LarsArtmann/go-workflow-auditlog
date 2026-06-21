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

// IsKnown returns true if the event type is a recognized value.
func (e EventType) IsKnown() bool {
	_, ok := eventTypeMeta[e]

	return ok
}

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

// IsKnown returns true if the phase is a recognized value.
func (p Phase) IsKnown() bool {
	return p == PhaseBefore || p == PhaseAfter
}

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
// Centralizing the label, icon, and color here keeps per-status presentation
// in one place and makes new statuses a one-line addition.
//
//nolint:gochecknoglobals // Lookup table, treated as immutable after init.
var stepStatusMeta = map[StepStatus]struct {
	Label     string
	Icon      string
	FillColor string
	FontColor string
}{
	StepStatusPending: {Label: "Pending", Icon: "\u26AA"},
	StepStatusRunning: {Label: "Running", Icon: "\U0001F7E1"},
	StepStatusSucceeded: {
		Label:     "Succeeded",
		Icon:      "\U0001F7E2",
		FillColor: statusFillSucceeded,
		FontColor: fontColorLight,
	},
	StepStatusFailed: {
		Label:     "Failed",
		Icon:      "\U0001F534",
		FillColor: statusFillFailed,
		FontColor: fontColorLight,
	},
	StepStatusCanceled: {
		Label:     "Canceled",
		Icon:      "\U0001F6AB",
		FillColor: statusFillCanceled,
		FontColor: fontColorLight,
	},
	StepStatusSkipped: {
		Label:     "Skipped",
		Icon:      "\u23ED\uFE0F",
		FillColor: statusFillSkipped,
		FontColor: fontColorDim,
	},
}

// Status color constants shared across all diagram and tree renderers.
const (
	statusFillSucceeded = "#2d5a2d" // green
	statusFillFailed    = "#8b2d2d" // red
	statusFillSkipped   = "#4a4a4a" // gray
	statusFillCanceled  = "#5a3d2d" // orange-brown

	fontColorLight = "#fff" // white text on dark fills
	fontColorDim   = "#ccc" // light gray for skipped (lower contrast)
)

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

// Color returns the fill and font colors for this step status, used by all
// diagram renderers (Mermaid, Graphviz, PlantUML, D2). Terminal statuses get
// colors; non-terminal statuses (pending/running) return empty strings (the
// renderer uses its default appearance).
func (s StepStatus) Color() (string, string) {
	if m, ok := stepStatusMeta[s]; ok {
		return m.FillColor, m.FontColor
	}

	return "", ""
}

// RunID identifies a single execution ("run") of a workflow. It is stamped on
// every Event and on the WorkflowReport so all observations from one execution
// can be correlated across systems (e.g. matched to a distributed trace).
//
// RunID is a branded string type: it serializes to/from JSON as a plain string
// but the type system prevents accidentally passing a WorkflowID (also a
// string) where a RunID is expected. Convert with RunID("value") or string(id).
type RunID string

// String returns the run ID as a plain string. Satisfies fmt.Stringer.
func (r RunID) String() string { return string(r) }

// IsEmpty returns true when the RunID is the zero value (no ID assigned).
func (r RunID) IsEmpty() bool { return r == "" }

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
