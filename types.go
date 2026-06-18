package auditlog

// SchemaVersion is the current report schema version.
const SchemaVersion = "0.1.0"

// EventType categorizes audit log events.
type EventType string

const (
	// EventTypeAttemptStart fires when a step attempt begins (each retry try).
	EventTypeAttemptStart EventType = "attempt_start"
	// EventTypeAttemptEnd fires when a step attempt finishes (each retry try).
	EventTypeAttemptEnd EventType = "attempt_end"
)

// Label returns the human-readable display label for this event type.
func (e EventType) Label() string {
	switch e {
	case EventTypeAttemptStart:
		return "Attempt Start"
	case EventTypeAttemptEnd:
		return "Attempt End"
	default:
		return ""
	}
}

// Color returns the CSS color token for this event type, used in HTML visualizations.
func (e EventType) Color() string {
	switch e {
	case EventTypeAttemptStart:
		return "var(--success)"
	case EventTypeAttemptEnd:
		return "var(--warning)"
	default:
		return ""
	}
}

// Phase indicates whether an event is the start or end of an operation.
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

// String returns the step status name.
func (s StepStatus) String() string { return string(s) }

// Label returns the human-readable display label for this step status.
func (s StepStatus) Label() string {
	switch s {
	case StepStatusPending:
		return "Pending"
	case StepStatusRunning:
		return "Running"
	case StepStatusSucceeded:
		return "Succeeded"
	case StepStatusFailed:
		return "Failed"
	case StepStatusCanceled:
		return "Canceled"
	case StepStatusSkipped:
		return "Skipped"
	default:
		return ""
	}
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
	switch s {
	case StepStatusPending:
		return "\u26AA"
	case StepStatusRunning:
		return "\U0001F7E1"
	case StepStatusSucceeded:
		return "\U0001F7E2"
	case StepStatusFailed:
		return "\U0001F534"
	case StepStatusCanceled:
		return "\U0001F6AB"
	case StepStatusSkipped:
		return "\u23ED\uFE0F"
	default:
		return ""
	}
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
