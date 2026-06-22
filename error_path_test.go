package auditlog_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// errWriteFail is the sentinel error returned by failingWriter.
var errWriteFail = errors.New("simulated I/O failure")

// failingWriter is an io.Writer that always returns an error.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errWriteFail
}

// minimalReport returns a small WorkflowReport with one step and one event,
// sufficient for all render/export methods to produce output.
func minimalReport() auditlog.WorkflowReport {
	now := time.Now()
	dur := 42.0

	return auditlog.WorkflowReport{
		Version:    auditlog.SchemaVersion,
		WorkflowID: "error-path-test",
		ExportedAt: now,
		EventCount: 1,
		StepCount:  1,
		Steps: []auditlog.StepInfo{
			{
				StepRef:      auditlog.StepRef{Name: "step-a", StepType: "TestStep"},
				Status:       auditlog.StepStatusSucceeded,
				AttemptCount: 1,
				DurationMs:   &dur,
				StartedAt:    &now,
				FinishedAt:   &now,
			},
		},
		Events: []auditlog.Event{
			{
				StepRef:    auditlog.StepRef{Name: "step-a", StepType: "TestStep"},
				Sequence:   1,
				Timestamp:  now,
				EventType:  auditlog.EventTypeAttemptStart,
				Phase:      auditlog.PhaseBefore,
				DurationMs: &dur,
				Status:     auditlog.StepStatusSucceeded,
			},
		},
	}
}

// =============================================================================
// P0-1: WriteJSON — failing writer triggers ErrRenderFailed
// (json.Encoder writes directly to the writer)
// =============================================================================

func TestErrorPath_WriteJSON_FailingWriter_ErrRenderFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.WriteJSON(failingWriter{})

	if !errors.Is(err, auditlog.ErrRenderFailed) {
		t.Errorf("WriteJSON(failingWriter) error = %v, want errors.Is(err, ErrRenderFailed)", err)
	}
}

// =============================================================================
// P0-2: WriteMermaid + WriteGraphviz — failing writer triggers ErrExportWriteFailed
// (render to string succeeds, write to writer fails)
// =============================================================================

func TestErrorPath_WriteMermaid_FailingWriter_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.WriteMermaid(failingWriter{})

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("WriteMermaid(failingWriter) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

func TestErrorPath_WriteGraphviz_FailingWriter_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.WriteGraphviz(failingWriter{})

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("WriteGraphviz(failingWriter) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

// =============================================================================
// P0-3: WritePlantUML + WriteD2 — failing writer triggers ErrExportWriteFailed
// =============================================================================

func TestErrorPath_WritePlantUML_FailingWriter_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.WritePlantUML(failingWriter{})

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("WritePlantUML(failingWriter) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

func TestErrorPath_WriteD2_FailingWriter_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.WriteD2(failingWriter{})

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("WriteD2(failingWriter) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

// =============================================================================
// P0-4: WriteTree + WriteHTMLTree — failing writer triggers ErrExportWriteFailed
// =============================================================================

func TestErrorPath_WriteTree_FailingWriter_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.WriteTree(failingWriter{})

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("WriteTree(failingWriter) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

func TestErrorPath_WriteHTMLTree_FailingWriter_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.WriteHTMLTree(failingWriter{})

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("WriteHTMLTree(failingWriter) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

// =============================================================================
// P0-5: WriteTable — failing writer triggers ErrRenderFailed
// (go-output RenderTableData writes directly to the writer)
// =============================================================================

func TestErrorPath_WriteTable_FailingWriter_ErrRenderFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.WriteTable(failingWriter{}, output.FormatMarkdown, output.RenderOptions{})

	if !errors.Is(err, auditlog.ErrRenderFailed) {
		t.Errorf("WriteTable(failingWriter) error = %v, want errors.Is(err, ErrRenderFailed)", err)
	}
}

// =============================================================================
// P0-6: WriteHTML — failing writer triggers ErrExportWriteFailed
// (marshal to bytes succeeds, writer.Write fails)
// =============================================================================

func TestErrorPath_WriteHTML_FailingWriter_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.WriteHTML(failingWriter{})

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("WriteHTML(failingWriter) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

// =============================================================================
// P0-6b: WriteNDJSON — failing writer triggers ErrExportWriteFailed
// (bufio buffers encode, flush to writer fails)
// =============================================================================

func TestErrorPath_WriteNDJSON_FailingWriter_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.WriteNDJSON(failingWriter{})

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("WriteNDJSON(failingWriter) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

// =============================================================================
// P0-7: Export* writeToFile — unwritable directory triggers ErrExportWriteFailed
// =============================================================================

func TestErrorPath_ExportJSON_UnwritableDir_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.ExportJSON("/nonexistent_dir_auditlog_test/output.json")

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("ExportJSON(unwritable) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

func TestErrorPath_ExportMermaid_UnwritableDir_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.ExportMermaid("/nonexistent_dir_auditlog_test/output.mmd")

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("ExportMermaid(unwritable) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

func TestErrorPath_ExportHTML_UnwritableDir_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.ExportHTML("/nonexistent_dir_auditlog_test/output.html")

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("ExportHTML(unwritable) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

func TestErrorPath_ExportD2_UnwritableDir_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	err := report.ExportD2("/nonexistent_dir_auditlog_test/output.d2")

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("ExportD2(unwritable) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

// =============================================================================
// P0-7b: Export via Auditor — writeToFile failure propagates through Auditor
// =============================================================================

func TestErrorPath_Auditor_ExportJSON_UnwritableDir_ErrExportWriteFailed(t *testing.T) {
	t.Parallel()

	a := mustNewWithID(t, "auditor-error-path")

	err := a.ExportJSON("/nonexistent_dir_auditlog_test/audit.json")
	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("Auditor.ExportJSON(unwritable) error = %v, want errors.Is(err, ErrExportWriteFailed)", err)
	}
}

// =============================================================================
// P0-8: LoadReport(nonexistent file) → ErrReportLoadFailed
// =============================================================================

func TestErrorPath_LoadReport_NonexistentFile_ErrReportLoadFailed(t *testing.T) {
	t.Parallel()

	_, err := auditlog.LoadReport("/nonexistent_dir_auditlog_test/missing_report.json")

	if !errors.Is(err, auditlog.ErrReportLoadFailed) {
		t.Errorf("LoadReport(nonexistent) error = %v, want errors.Is(err, ErrReportLoadFailed)", err)
	}
}

// =============================================================================
// P0-9: LoadReportFromReader(bad JSON) + LoadReportFromBytes(garbage) → ErrReportLoadFailed
// =============================================================================

func TestErrorPath_LoadReportFromReader_BadJSON_ErrReportLoadFailed(t *testing.T) {
	t.Parallel()

	_, err := auditlog.LoadReportFromReader(strings.NewReader("{ this is not valid json"))

	if !errors.Is(err, auditlog.ErrReportLoadFailed) {
		t.Errorf("LoadReportFromReader(bad json) error = %v, want errors.Is(err, ErrReportLoadFailed)", err)
	}
}

func TestErrorPath_LoadReportFromBytes_Garbage_ErrReportLoadFailed(t *testing.T) {
	t.Parallel()

	_, err := auditlog.LoadReportFromBytes([]byte("not json at all"))

	if !errors.Is(err, auditlog.ErrReportLoadFailed) {
		t.Errorf("LoadReportFromBytes(garbage) error = %v, want errors.Is(err, ErrReportLoadFailed)", err)
	}
}

// =============================================================================
// P0-9b: Round-trip — successfully loaded report is usable (positive control)
// Ensures the Load* path works correctly when data is valid.
// =============================================================================

func TestErrorPath_LoadReportFromBytes_ValidJSON_Succeeds(t *testing.T) {
	t.Parallel()

	report := minimalReport()

	var buf strings.Builder

	err := report.WriteJSON(&buf)
	if err != nil {
		t.Fatalf("WriteJSON setup error: %v", err)
	}

	loaded, err := auditlog.LoadReportFromBytes([]byte(buf.String()))
	if err != nil {
		t.Fatalf("LoadReportFromBytes(valid) error = %v, want nil", err)
	}

	if loaded.WorkflowID != report.WorkflowID {
		t.Errorf("loaded WorkflowID = %q, want %q", loaded.WorkflowID, report.WorkflowID)
	}
}
