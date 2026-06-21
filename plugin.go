package auditlog

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	flow "github.com/Azure/go-workflow"
	"github.com/larsartmann/go-output"
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
	// RunID is an optional identifier for a single execution ("run") of the
	// workflow. It is stamped on every Event and on the WorkflowReport so that
	// all observations from one execution can be correlated across systems
	// (e.g. matched to a distributed trace). If empty, New() generates a random
	// 128-bit hex ID.
	RunID RunID
	// OnEvent is called after each event is captured, outside the recorder
	// lock so it cannot deadlock the recorder. Must not block.
	// Note: concurrent steps invoke this concurrently — the callback must be
	// goroutine-safe (e.g. guard shared state with a mutex). Nil disables it.
	OnEvent func(Event)
	// MaxEvents caps the number of events stored in memory. When 0 (default),
	// events grow without bound. When > 0, the recorder stops appending new
	// events after reaching the cap and increments DroppedEventCount.
	MaxEvents int
	// InitialEventCapacity pre-allocates the events slice to avoid runtime
	// reallocations. When 0, defaults to 256.
	InitialEventCapacity int
}

// ErrWorkflowIDPathSep is returned by [Config.Validate] (and thus [New]) when
// Config.WorkflowID contains a path separator, which would break file-based
// export paths. Consumers can match on it with [errors.Is].
var ErrWorkflowIDPathSep = errors.New("config.WorkflowID must not contain path separators")

// Validate returns an error if the config is invalid.
func (c Config) Validate() error {
	if strings.ContainsAny(c.WorkflowID, "/\\") {
		return fmt.Errorf("%w: %q", ErrWorkflowIDPathSep, c.WorkflowID)
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

	if config.RunID == "" {
		config.RunID = newRunID()
	}

	err := config.Validate()
	if err != nil {
		return nil, err
	}

	if !config.Enabled {
		config.Enabled = envIsEnabled()
	}

	recorder := NewRecorder(config.WorkflowID, config.RunID, config.OnEvent)
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

// RunID returns the run identifier stamped on every captured event. Useful for
// correlating the audit log with external systems (traces, logs) before a full
// report is built.
func (a *Auditor) RunID() RunID {
	return a.recorder.RunID()
}

// DroppedEventCount returns the number of events dropped due to Config.MaxEvents.
func (a *Auditor) DroppedEventCount() int64 {
	return a.recorder.DroppedEventCount()
}

// WriteJSON writes the full WorkflowReport as indented JSON to writer.
func (a *Auditor) WriteJSON(writer io.Writer) error {
	return a.Report().WriteJSON(writer)
}

// WriteReportJSON writes the full WorkflowReport as indented JSON to writer.
//
// Deprecated: Use WriteJSON for naming consistency with the diagram/table/tree
// Write* methods. This alias is kept for backward compatibility.
func (a *Auditor) WriteReportJSON(writer io.Writer) error {
	return a.WriteJSON(writer)
}

// WriteNDJSON writes every captured event as line-delimited JSON to writer.
func (a *Auditor) WriteNDJSON(writer io.Writer) error {
	return a.Report().WriteNDJSON(writer)
}

// WriteEventsNDJSON writes every captured event as line-delimited JSON to writer.
//
// Deprecated: Use WriteNDJSON for naming consistency. This alias is kept for
// backward compatibility.
func (a *Auditor) WriteEventsNDJSON(writer io.Writer) error {
	return a.WriteNDJSON(writer)
}

// ExportJSON writes the full WorkflowReport as indented JSON to path.
func (a *Auditor) ExportJSON(path string) error {
	return writeToFile(path, a.WriteJSON)
}

// ExportToFile writes the full WorkflowReport as indented JSON to path.
//
// Deprecated: Use ExportJSON for naming consistency with the diagram/table/tree
// Export* methods. This alias is kept for backward compatibility.
func (a *Auditor) ExportToFile(path string) error {
	return a.ExportJSON(path)
}

// ExportNDJSON writes every event as NDJSON to path.
func (a *Auditor) ExportNDJSON(path string) error {
	return writeToFile(path, a.WriteNDJSON)
}

// ExportEventsToNDJSON writes every event as NDJSON to path.
//
// Deprecated: Use ExportNDJSON for naming consistency. This alias is kept for
// backward compatibility.
func (a *Auditor) ExportEventsToNDJSON(path string) error {
	return a.ExportNDJSON(path)
}

// WriteMermaid writes the step DAG as a Mermaid diagram to the writer.
func (a *Auditor) WriteMermaid(writer io.Writer) error {
	return a.Report().WriteMermaid(writer)
}

// WritePlantUML writes the step DAG as a PlantUML diagram to the writer.
func (a *Auditor) WritePlantUML(writer io.Writer) error {
	return a.Report().WritePlantUML(writer)
}

// WriteGraphviz writes the step DAG as a Graphviz DOT diagram to the writer.
func (a *Auditor) WriteGraphviz(writer io.Writer) error {
	return a.Report().WriteGraphviz(writer)
}

// ExportMermaid writes the step DAG as Mermaid to path.
func (a *Auditor) ExportMermaid(path string) error {
	return writeToFile(path, a.WriteMermaid)
}

// ExportPlantUML writes the step DAG as PlantUML to path.
func (a *Auditor) ExportPlantUML(path string) error {
	return writeToFile(path, a.WritePlantUML)
}

// ExportGraphviz writes the step DAG as Graphviz DOT to path.
func (a *Auditor) ExportGraphviz(path string) error {
	return writeToFile(path, a.WriteGraphviz)
}

// WriteD2 writes the step DAG as a D2 diagram to the writer.
func (a *Auditor) WriteD2(writer io.Writer) error {
	return a.Report().WriteD2(writer)
}

// ExportD2 writes the step DAG as D2 to path.
func (a *Auditor) ExportD2(path string) error {
	return writeToFile(path, a.WriteD2)
}

// WriteTable writes the step summary as a table in the specified format.
func (a *Auditor) WriteTable(writer io.Writer, format output.Format, opts output.RenderOptions) error {
	return a.Report().WriteTable(writer, format, opts)
}

// ExportTable writes the step summary table to path in the specified format.
func (a *Auditor) ExportTable(path string, format output.Format, opts output.RenderOptions) error {
	return writeToFile(path, func(w io.Writer) error {
		return a.WriteTable(w, format, opts)
	})
}

// WriteTree writes the step DAG as an ASCII tree to the writer.
func (a *Auditor) WriteTree(writer io.Writer) error {
	return a.Report().WriteTree(writer)
}

// ExportTree writes the step DAG as an ASCII tree to path.
func (a *Auditor) ExportTree(path string) error {
	return writeToFile(path, a.WriteTree)
}

// WriteHTMLTree writes the step DAG as an HTML nested list tree to the writer.
func (a *Auditor) WriteHTMLTree(writer io.Writer) error {
	return a.Report().WriteHTMLTree(writer)
}

// ExportHTMLTree writes the step DAG as an HTML tree to path.
func (a *Auditor) ExportHTMLTree(path string) error {
	return writeToFile(path, a.WriteHTMLTree)
}

// --- String-variant methods (mirror WorkflowReport.Write*String) ---

// WriteMermaidString returns the Mermaid diagram as a string.
func (a *Auditor) WriteMermaidString() (string, error) {
	return a.Report().WriteMermaidString()
}

// WritePlantUMLString returns the PlantUML diagram as a string.
func (a *Auditor) WritePlantUMLString() (string, error) {
	return a.Report().WritePlantUMLString()
}

// WriteGraphvizString returns the Graphviz DOT diagram as a string.
func (a *Auditor) WriteGraphvizString() (string, error) {
	return a.Report().WriteGraphvizString()
}

// WriteD2String returns the D2 diagram as a string.
func (a *Auditor) WriteD2String() (string, error) {
	return a.Report().WriteD2String()
}

// WriteTableString returns the step summary table as a string in the given format.
func (a *Auditor) WriteTableString(format output.Format, opts output.RenderOptions) (string, error) {
	return a.Report().WriteTableString(format, opts)
}

// WriteTreeString returns the ASCII tree as a string.
func (a *Auditor) WriteTreeString() (string, error) {
	return a.Report().WriteTreeString()
}

// WriteHTMLTreeString returns the HTML tree as a string.
func (a *Auditor) WriteHTMLTreeString() (string, error) {
	return a.Report().WriteHTMLTreeString()
}

// fileWriteBufferSize is the bufio buffer size used for atomic file exports.
const fileWriteBufferSize = 65536

// writeToFile creates a file at path and calls fn with a buffered writer.
// The bufio.Writer batches small writes into 64KB blocks, reducing syscall count
// by 10-100x compared to writing directly to os.File.
//
// Writes are atomic: data is written to a temporary file in the same directory,
// then atomically renamed to the final path. A crash during write leaves the
// previous file (if any) intact rather than a partial file.
func writeToFile(path string, fn func(io.Writer) error) error {
	dir := filepath.Dir(path)

	tmpFile, err := os.CreateTemp(dir, ".tmp-auditlog-*")
	if err != nil {
		return fmt.Errorf("create temp file in %q: %w", dir, err)
	}

	tmpPath := tmpFile.Name()
	cleanup := true

	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	bw := bufio.NewWriterSize(tmpFile, fileWriteBufferSize)

	writeErr := fn(bw)

	flushErr := bw.Flush()

	closeErr := tmpFile.Close()

	if writeErr != nil {
		return writeErr
	}

	if flushErr != nil {
		return fmt.Errorf("flush temp file %q: %w", tmpPath, flushErr)
	}

	if closeErr != nil {
		return fmt.Errorf("close temp file %q: %w", tmpPath, closeErr)
	}

	renameErr := os.Rename(tmpPath, path)
	if renameErr != nil {
		return fmt.Errorf("rename %q → %q: %w", tmpPath, path, renameErr)
	}

	cleanup = false

	return nil
}
