package auditlog

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	flow "github.com/Azure/go-workflow"
)

// EnvKeyEnabled is the environment variable that controls audit logging.
// Set to "true", "1", or "yes" to enable. Any other value (or unset) disables it.
const EnvKeyEnabled = "WORKFLOW_AUDITLOG_ENABLED"

// Config controls the audit log behaviour.
type Config struct {
	// Enabled turns audit logging on or off. When false the Auditor is a no-op.
	// If left as zero-value (false), New() checks the WORKFLOW_AUDITLOG_ENABLED env var.
	Enabled bool
	// WorkflowID is an optional human-readable identifier for the workflow.
	WorkflowID string
	// OnEvent is called after each event is captured. Must not block.
	// Called sequentially in callback order. Nil disables the callback.
	OnEvent func(Event)
	// MaxEvents caps the number of events stored in memory. When 0 (default),
	// events grow without bound. When > 0, the recorder stops appending new
	// events after reaching the cap and increments DroppedEventCount.
	MaxEvents int
	// InitialEventCapacity pre-allocates the events slice to avoid runtime
	// reallocations. When 0, defaults to 256.
	InitialEventCapacity int
}

var errWorkflowIDPathSep = errors.New("config.WorkflowID must not contain path separators")

// Validate returns an error if the config is invalid.
func (c Config) Validate() error {
	if strings.ContainsAny(c.WorkflowID, "/\\") {
		return fmt.Errorf("%w: %q", errWorkflowIDPathSep, c.WorkflowID)
	}

	return nil
}

const defaultWorkflowID = "default"

// Auditor wraps a [flow.Workflow] with audit logging.
type Auditor struct {
	recorder *Recorder
	config   Config
	beforeFn flow.BeforeStep
	afterFn  flow.AfterStep
}

// New creates an audit log Auditor.
//
// When Config.Enabled is false (the zero value), New checks the
// WORKFLOW_AUDITLOG_ENABLED environment variable. Set it to "true", "1", or
// "yes" to enable audit logging without changing code.
//
// If WorkflowID is empty it defaults to "default".
//
// Returns an error if Config.Validate() fails.
func New(config Config) (*Auditor, error) {
	if config.WorkflowID == "" {
		config.WorkflowID = defaultWorkflowID
	}

	err := config.Validate()
	if err != nil {
		return nil, err
	}

	if !config.Enabled {
		config.Enabled = envIsEnabled()
	}

	recorder := NewRecorder(config.WorkflowID, config.OnEvent)
	if config.MaxEvents > 0 {
		recorder.maxEvents = config.MaxEvents
	}

	if config.InitialEventCapacity > 0 && len(recorder.events) == 0 {
		recorder.events = make([]Event, 0, config.InitialEventCapacity)
	}

	before, after := recorder.makeCallbacks()

	return &Auditor{
		recorder: recorder,
		config:   config,
		beforeFn: before,
		afterFn:  after,
	}, nil
}

func envIsEnabled() bool {
	switch os.Getenv(EnvKeyEnabled) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

// Report returns a consolidated snapshot of everything observed so far.
func (a *Auditor) Report() WorkflowReport {
	return a.recorder.BuildReport()
}

// Events returns a defensive copy of all captured events.
func (a *Auditor) Events() []Event {
	return a.recorder.Events()
}

// EventsCount returns the number of captured events without copying the slice.
func (a *Auditor) EventsCount() int {
	return a.recorder.EventsCount()
}

// DroppedEventCount returns the number of events dropped due to Config.MaxEvents.
func (a *Auditor) DroppedEventCount() int64 {
	return a.recorder.DroppedEventCount()
}

// WriteReportJSON writes the full WorkflowReport as indented JSON to writer.
func (a *Auditor) WriteReportJSON(writer io.Writer) error {
	return a.Report().WriteJSON(writer)
}

// WriteEventsNDJSON writes every captured event as line-delimited JSON to writer.
func (a *Auditor) WriteEventsNDJSON(writer io.Writer) error {
	return a.Report().WriteNDJSON(writer)
}

// ExportToFile writes the full WorkflowReport as indented JSON to path.
func (a *Auditor) ExportToFile(path string) error {
	return writeToFile(path, a.WriteReportJSON)
}

// ExportEventsToNDJSON writes every event as NDJSON to path.
func (a *Auditor) ExportEventsToNDJSON(path string) error {
	return writeToFile(path, a.WriteEventsNDJSON)
}

// WriteMermaid writes the step DAG as a Mermaid diagram to the writer.
func (a *Auditor) WriteMermaid(writer io.Writer) error {
	return a.Report().WriteMermaid(writer)
}

// WritePlantUML writes the step DAG as a PlantUML diagram to the writer.
func (a *Auditor) WritePlantUML(writer io.Writer) error {
	return a.Report().WritePlantUML(writer)
}

// ExportMermaid writes the step DAG as Mermaid to path.
func (a *Auditor) ExportMermaid(path string) error {
	return writeToFile(path, a.WriteMermaid)
}

// ExportPlantUML writes the step DAG as PlantUML to path.
func (a *Auditor) ExportPlantUML(path string) error {
	return writeToFile(path, a.WritePlantUML)
}

// writeToFile is a helper that creates a file, calls the writer function, and
// properly closes the file, returning the write error if any.
func writeToFile(path string, write func(io.Writer) error) error {
	f, err := os.Create(path) //nolint:gosec // path is user-provided by design.
	if err != nil {
		return fmt.Errorf("create file %q: %w", path, err)
	}

	writeErr := write(f)
	closeErr := f.Close()

	if writeErr != nil {
		return writeErr
	}

	if closeErr != nil {
		return fmt.Errorf("close file %q: %w", path, closeErr)
	}

	return nil
}
