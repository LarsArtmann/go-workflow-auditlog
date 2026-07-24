package auditlog_test

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

func TestReport_WriteCSV(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "csv-step")

	err := a.Report().WriteCSV(buf)
	if err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty CSV output")
	}

	reader := csv.NewReader(strings.NewReader(output))

	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) < 2 {
		t.Fatalf("expected at least header + 1 row, got %d rows", len(records))
	}

	header := records[0]
	expectedCols := []string{
		"step_id", "step_name", "step_type", "status",
		"attempt_count", "max_attempts",
		"started_at", "finished_at", "duration_ms",
		"has_retry", "has_timeout", "error",
		"dependencies", "dependents",
	}

	for i, col := range expectedCols {
		if i >= len(header) {
			t.Errorf("header missing column %q", col)

			break
		}

		if header[i] != col {
			t.Errorf("header[%d]: expected %q, got %q", i, col, header[i])
		}
	}

	firstRow := records[1]
	if firstRow[1] != "csv-step" {
		t.Errorf("expected step_name=csv-step, got %q", firstRow[1])
	}

	if firstRow[3] != "succeeded" {
		t.Errorf("expected status=succeeded, got %q", firstRow[3])
	}
}

func TestReport_WriteTSV(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "tsv-step")

	err := a.Report().WriteTSV(buf)
	if err != nil {
		t.Fatalf("WriteTSV: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty TSV output")
	}

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least header + 1 row, got %d lines", len(lines))
	}

	header := strings.Split(lines[0], "\t")
	if len(header) != 14 {
		t.Errorf("expected 14 columns in header, got %d", len(header))
	}

	firstRow := strings.Split(lines[1], "\t")
	if firstRow[1] != "tsv-step" {
		t.Errorf("expected step_name=tsv-step, got %q", firstRow[1])
	}
}

func TestReport_WriteCSV_FailingWriter(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "csv-fail-step")

	err := a.Report().WriteCSV(testhelpers.FailingWriter{})
	if err == nil {
		t.Fatal("expected error from FailingWriter")
	}

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("expected ErrExportWriteFailed, got: %v", err)
	}
}

func TestReport_ExportCSV(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "csv-export-step", "steps.csv")

	err := a.Report().ExportCSV(path)
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
}

func TestReport_ExportTSV(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "tsv-export-step", "steps.tsv")

	err := a.Report().ExportTSV(path)
	if err != nil {
		t.Fatalf("ExportTSV: %v", err)
	}
}

// ExampleWorkflowReport_WriteCSV demonstrates exporting all steps as CSV
// for spreadsheet analysis or data pipelines. Pointer fields (timestamps,
// duration, error) render as empty strings when nil.
func ExampleWorkflowReport_WriteCSV() {
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{
				StepRef: auditlog.StepRef{Name: "fetch"},
				StepID:  1,
				Status:  auditlog.StepStatusSucceeded,
			},
		},
	}

	_ = report.WriteCSV(os.Stdout)

	// Output: step_id,step_name,step_type,status,attempt_count,max_attempts,started_at,finished_at,duration_ms,has_retry,has_timeout,error,dependencies,dependents
	// 1,fetch,,succeeded,0,0,,,,false,false,,,
}

func BenchmarkWriteCSV_LargeReport(b *testing.B) {
	steps := make([]auditlog.StepInfo, 0, 100)

	for i := range 100 {
		dur := float64(i * 100)

		steps = append(steps, auditlog.StepInfo{
			StepRef:      auditlog.StepRef{Name: fmt.Sprintf("step-%04d", i), StepType: "BenchStep"},
			StepID:       i + 1,
			Status:       auditlog.StepStatusSucceeded,
			AttemptCount: 1,
			HasRetry:     i%3 == 0,
			HasTimeout:   i%5 == 0,
			DurationMs:   &dur,
		})
	}

	report := auditlog.WorkflowReport{
		Version:        auditlog.SchemaVersion,
		WorkflowID:     "bench-csv",
		StepCount:      100,
		SucceededCount: 100,
		Steps:          steps,
	}

	b.ResetTimer()

	for b.Loop() {
		err := report.WriteCSV(io.Discard)
		if err != nil {
			b.Fatalf("WriteCSV: %v", err)
		}
	}
}
