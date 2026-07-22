// Package viz renders auditlog reports into diagrams, tables, trees, and an
// interactive HTML dashboard. It depends on the core auditlog module and on
// github.com/larsartmann/go-output for rendering.
package viz

import (
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

// Re-export WriteToFile so viz exporters can use the same atomic write helper
// as the core package.
var WriteToFile = auditlog.WriteToFile
