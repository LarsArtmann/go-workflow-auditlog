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
	t.Parallel() //art-dupl:accept idiomatic Go test boilerplate

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
		"failure_reason",
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
	t.Parallel() //art-dupl:accept idiomatic Go test boilerplate

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
	if len(header) != 15 {
		t.Errorf("expected 15 columns in header, got %d", len(header))
	}

	firstRow := strings.Split(lines[1], "\t")
	if firstRow[1] != "tsv-step" {
		t.Errorf("expected step_name=tsv-step, got %q", firstRow[1])
	}
}

func TestReport_WriteCSV_FailingWriter(t *testing.T) {
	t.Parallel() //art-dupl:accept idiomatic Go test boilerplate

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

// TestReport_WriteCSV_SpecialChars_RoundTrip verifies that encoding/csv RFC
// 4180 quoting preserves step names and dependency names containing commas,
// double quotes, embedded newlines, tabs, and unicode through a WriteCSV →
// csv.Read round-trip. This is a structural-integrity regression test: any
// change to the delimited writer that breaks quoting would corrupt these
// fields when opened in spreadsheet tools or re-parsed.
func TestReport_WriteCSV_SpecialChars_RoundTrip(t *testing.T) {
	t.Parallel()

	names := []string{
		"plain",
		"has,comma",
		`has"doublequote`,
		"has\nembedded-newline",
		"has\ttab",
		`combo"a,b\nc`,
		"unicode: Ω🎨中文日本語", //nolint:gosmopolitan // deliberate unicode round-trip fixture
		"semicolon;name",   // ; is the deps separator, but quoting keeps the NAME cell intact
	}

	steps := make([]auditlog.StepInfo, 0, len(names))

	for i, name := range names {
		step := auditlog.StepInfo{
			StepRef: auditlog.StepRef{Name: name, StepType: "TrickyStep"},
			StepID:  i + 1,
			Status:  auditlog.StepStatusSucceeded,
		}

		// Wire a dependency whose NAME carries special CSV chars (comma, quote,
		// newline) to prove the semicolon-joined dependencies cell is itself
		// quoted. Names containing ';' are excluded here because ';' is the
		// in-cell separator and cannot be distinguished from a dep boundary —
		// see TestReport_WriteCSV_DependencySemicolonCollision.
		if i > 0 && !strings.Contains(names[i-1], ";") {
			step.Dependencies = []auditlog.StepRef{
				{Name: names[i-1], StepType: "TrickyStep"},
			}
		}

		steps = append(steps, step)
	}

	report := auditlog.WorkflowReport{Steps: steps}

	var buf strings.Builder

	err := report.WriteCSV(&buf)
	if err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != len(names)+1 {
		t.Fatalf("expected %d records (header + rows), got %d", len(names)+1, len(records))
	}

	const (
		nameCol = 1  // step_name
		depCol  = 13 // dependencies
	)

	for i, want := range names {
		row := records[i+1]

		if got := row[nameCol]; got != want {
			t.Errorf("step %d name round-trip mismatch:\n got %q\nwant %q", i, got, want)
		}

		if i > 0 && !strings.Contains(names[i-1], ";") {
			if gotDep := row[depCol]; gotDep != names[i-1] {
				t.Errorf("step %d dependency round-trip mismatch:\n got %q\nwant %q", i, gotDep, names[i-1])
			}
		}
	}
}

// TestReport_WriteCSV_FormulaVectors_PreservedVerbatim documents that CSV
// formula-injection vectors (=cmd, +cmd, -cmd, @cmd) in step names are
// exported VERBATIM, without neutralization (e.g. no leading-quote prefix).
//
// This is intentional and correct for an AUDIT library: the export must
// faithfully reproduce recorded data. Sanitizing step names here would
// falsify the audit trail. CSV-injection hardening belongs at spreadsheet
// open-time (the consuming tool's responsibility), not in the data-export
// layer of a truthful audit log.
//
// The test locks in verbatim preservation so a future change that silently
// drops or mangles these characters is caught.
func TestReport_WriteCSV_FormulaVectors_PreservedVerbatim(t *testing.T) {
	t.Parallel()

	formulaVectors := []string{
		"=HYPERLINK(\"http://example.com\",\"click\")",
		"+1+1",
		"-2+3",
		"@SUM(A1:A2)",
		"=cmd|'/c calc'!A1",
	}

	steps := make([]auditlog.StepInfo, 0, len(formulaVectors))

	for i, name := range formulaVectors {
		steps = append(steps, auditlog.StepInfo{
			StepRef: auditlog.StepRef{Name: name, StepType: "FormulaStep"},
			StepID:  i + 1,
			Status:  auditlog.StepStatusFailed,
		})
	}

	report := auditlog.WorkflowReport{Steps: steps}

	var buf strings.Builder

	err := report.WriteCSV(&buf)
	if err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	const nameCol = 1

	for i, want := range formulaVectors {
		if got := records[i+1][nameCol]; got != want {
			t.Errorf("formula vector %d not preserved verbatim:\n got %q\nwant %q", i, got, want)
		}
	}
}

// TestReport_WriteCSV_DependencySemicolonCollision documents a known
// limitation: dependencies and dependents are rendered as a single
// semicolon-separated cell, so a dependency NAME containing ';' cannot be
// distinguished from the separator on re-parse. The step NAME itself
// round-trips ';' correctly (CSV quoting handles it); only the joined
// multi-value cells are affected.
func TestReport_WriteCSV_DependencySemicolonCollision(t *testing.T) {
	t.Parallel()

	withSemicolon := "dep;with;semicolons"

	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{
				StepRef: auditlog.StepRef{Name: "root", StepType: "Step"},
				StepID:  1,
				Status:  auditlog.StepStatusSucceeded,
				Dependencies: []auditlog.StepRef{
					{Name: withSemicolon, StepType: "Step"},
				},
			},
		},
	}

	var buf strings.Builder

	err := report.WriteCSV(&buf)
	if err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	const depCol = 13

	depCell := records[1][depCol]

	// The raw cell content preserves the name verbatim...
	if !strings.Contains(depCell, withSemicolon) {
		t.Fatalf("dependency cell lost the name: got %q", depCell)
	}

	// ...but a naive split on ';' would fragment it into 3 parts, proving the
	// collision. Consumers that need lossless dependency lists must use the
	// JSON/NDJSON export, not the delimited formats.
	parts := strings.Split(depCell, ";")
	if len(parts) != 3 {
		t.Errorf("expected ';' collision to fragment into 3 parts, got %d: %v", len(parts), parts)
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

	// Output: step_id,step_name,step_type,status,attempt_count,max_attempts,started_at,finished_at,duration_ms,has_retry,has_timeout,error,failure_reason,dependencies,dependents
	// 1,fetch,,succeeded,0,0,,,,false,false,,,,
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
