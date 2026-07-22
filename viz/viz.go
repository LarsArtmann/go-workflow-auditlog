// Package viz renders auditlog reports into diagrams, tables, trees, and an
// interactive HTML dashboard. It depends on the core auditlog module and on
// github.com/larsartmann/go-output for rendering.
package viz

import (
	"io"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// Type aliases keep the viz API readable: callers inside package viz can use
// the unqualified names while still operating on core auditlog types.
type (
	// WorkflowReport is an alias for the core report type.
	WorkflowReport = auditlog.WorkflowReport
	// StepInfo is an alias for the core step info type.
	StepInfo = auditlog.StepInfo
	// StepStatus is an alias for the core step status enum.
	StepStatus = auditlog.StepStatus
	// StepRef is an alias for the core step reference type.
	StepRef = auditlog.StepRef
	// RunID is an alias for the core run identifier type.
	RunID = auditlog.RunID
	// Event is an alias for the core event type.
	Event = auditlog.Event
	// EventType is an alias for the core event type enum.
	EventType = auditlog.EventType
	// Phase is an alias for the core phase enum.
	Phase = auditlog.Phase
)

// Re-export core status constants so viz code can use unqualified names.
const (
	StepStatusPending   = auditlog.StepStatusPending
	StepStatusRunning   = auditlog.StepStatusRunning
	StepStatusSucceeded = auditlog.StepStatusSucceeded
	StepStatusFailed    = auditlog.StepStatusFailed
	StepStatusCanceled  = auditlog.StepStatusCanceled
	StepStatusSkipped   = auditlog.StepStatusSkipped
)

// Re-export core event-type constants so viz code can use unqualified names.
const (
	EventTypeAttemptStart = auditlog.EventTypeAttemptStart
	EventTypeAttemptEnd   = auditlog.EventTypeAttemptEnd
)

// Re-export core phase constants so viz code can use unqualified names.
const (
	PhaseBefore = auditlog.PhaseBefore
	PhaseAfter  = auditlog.PhaseAfter
)

// Re-export sentinel errors so viz code can use unqualified names.
var (
	// ErrRenderFailed wraps rendering or marshaling failures.
	ErrRenderFailed = auditlog.ErrRenderFailed
	// ErrExportWriteFailed wraps file-write failures during export.
	ErrExportWriteFailed = auditlog.ErrExportWriteFailed
)

// WriteToFile re-exports the core atomic file-write helper so viz exporters can
// write files the same way the core package does.
func WriteToFile(path string, fn func(io.Writer) error) error {
	return auditlog.WriteToFile(path, fn)
}

// AllStepStatuses returns every known StepStatus value in canonical order.
// Re-exported so viz code can enumerate statuses without importing core.
func AllStepStatuses() []StepStatus {
	return auditlog.AllStepStatuses()
}

// AllEventTypes returns every known EventType value in canonical order.
// Re-exported so viz code can enumerate event types without importing core.
func AllEventTypes() []EventType {
	return auditlog.AllEventTypes()
}
