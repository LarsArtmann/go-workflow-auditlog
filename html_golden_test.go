package auditlog_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// goldenExportedAt is a fixed timestamp so the rendered HTML is byte-for-byte
// reproducible across runs and machines.
var goldenExportedAt = time.Date(2026, 1, 15, 9, 30, 0, 0, time.UTC)

// goldenDurations provides stable duration pointers for the golden report.
var (
	goldenFetchMs     float64 = 5.2
	goldenTransformMs float64 = 8.7
	goldenSaveMs      float64 = 3.1
)

// goldenHTMLReport builds a deterministic, valid WorkflowReport with a
// fetch → transform → save pipeline and fixed timestamps so the golden file
// is byte-stable across runs.
func goldenHTMLReport() auditlog.WorkflowReport {
	fetchStarted := goldenExportedAt
	fetchFinished := goldenExportedAt.Add(5 * time.Millisecond)
	transformStarted := fetchFinished
	transformFinished := transformStarted.Add(9 * time.Millisecond)
	saveStarted := transformFinished
	saveFinished := saveStarted.Add(3 * time.Millisecond)

	return auditlog.WorkflowReport{
		Version:             auditlog.SchemaVersion,
		WorkflowID:          "golden-pipeline",
		RunID:               "abcdef0123456789abcdef0123456789",
		ExportedAt:          goldenExportedAt,
		StepCount:           3,
		SucceededCount:      3,
		EventCount:          6,
		WallClockDurationMs: 17.0,
		TotalDurationMs:     17.0,
		Steps: []auditlog.StepInfo{
			{
				StepRef:      auditlog.StepRef{Name: "fetch", StepType: "FetchStep"},
				StepID:       1,
				Status:       auditlog.StepStatusSucceeded,
				AttemptCount: 1,
				StartedAt:    &fetchStarted,
				FinishedAt:   &fetchFinished,
				DurationMs:   &goldenFetchMs,
				Dependents:   []auditlog.StepRef{{Name: "transform"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "transform", StepType: "TransformStep"},
				StepID:       2,
				Status:       auditlog.StepStatusSucceeded,
				AttemptCount: 1,
				StartedAt:    &transformStarted,
				FinishedAt:   &transformFinished,
				DurationMs:   &goldenTransformMs,
				Dependencies: []auditlog.StepRef{{Name: "fetch"}},
				Dependents:   []auditlog.StepRef{{Name: "save"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "save", StepType: "SaveStep"},
				StepID:       3,
				Status:       auditlog.StepStatusSucceeded,
				AttemptCount: 1,
				StartedAt:    &saveStarted,
				FinishedAt:   &saveFinished,
				DurationMs:   &goldenSaveMs,
				Dependencies: []auditlog.StepRef{{Name: "transform"}},
			},
		},
		Events: []auditlog.Event{
			{
				StepRef:   auditlog.StepRef{Name: "fetch", StepType: "FetchStep"},
				RunID:     "abcdef0123456789abcdef0123456789",
				Sequence:  1,
				Timestamp: fetchStarted,
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
				Attempt:   1,
			},
			{
				StepRef:    auditlog.StepRef{Name: "fetch", StepType: "FetchStep"},
				RunID:      "abcdef0123456789abcdef0123456789",
				Sequence:   2,
				Timestamp:  fetchFinished,
				EventType:  auditlog.EventTypeAttemptEnd,
				Phase:      auditlog.PhaseAfter,
				Attempt:    1,
				DurationMs: &goldenFetchMs,
				Status:     auditlog.StepStatusSucceeded,
			},
			{
				StepRef:   auditlog.StepRef{Name: "transform", StepType: "TransformStep"},
				RunID:     "abcdef0123456789abcdef0123456789",
				Sequence:  3,
				Timestamp: transformStarted,
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
				Attempt:   1,
			},
			{
				StepRef:    auditlog.StepRef{Name: "transform", StepType: "TransformStep"},
				RunID:      "abcdef0123456789abcdef0123456789",
				Sequence:   4,
				Timestamp:  transformFinished,
				EventType:  auditlog.EventTypeAttemptEnd,
				Phase:      auditlog.PhaseAfter,
				Attempt:    1,
				DurationMs: &goldenTransformMs,
				Status:     auditlog.StepStatusSucceeded,
			},
			{
				StepRef:   auditlog.StepRef{Name: "save", StepType: "SaveStep"},
				RunID:     "abcdef0123456789abcdef0123456789",
				Sequence:  5,
				Timestamp: saveStarted,
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
				Attempt:   1,
			},
			{
				StepRef:    auditlog.StepRef{Name: "save", StepType: "SaveStep"},
				RunID:      "abcdef0123456789abcdef0123456789",
				Sequence:   6,
				Timestamp:  saveFinished,
				EventType:  auditlog.EventTypeAttemptEnd,
				Phase:      auditlog.PhaseAfter,
				Attempt:    1,
				DurationMs: &goldenSaveMs,
				Status:     auditlog.StepStatusSucceeded,
			},
		},
	}
}

// TestReport_WriteHTML_GoldenFile renders the deterministic golden report to
// HTML and compares it against the committed golden file. Run with
// UPDATE_GOLDEN=1 to regenerate testdata/golden/report.html.
func TestReport_WriteHTML_GoldenFile(t *testing.T) {
	t.Parallel()

	report := goldenHTMLReport()

	var buf bytes.Buffer

	err := report.WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	got := buf.Bytes()
	goldenPath := filepath.Join("testdata", "golden", "report.html")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		err := os.MkdirAll(filepath.Dir(goldenPath), 0o755)
		if err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		err = os.WriteFile(goldenPath, got, 0o644)
		if err != nil {
			t.Fatalf("write golden: %v", err)
		}

		t.Skipf("golden file updated: %s", goldenPath)

		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file (%s): %v\n"+
			"hint: run UPDATE_GOLDEN=1 go test -run %s to create it",
			goldenPath, err, t.Name())
	}

	if !bytes.Equal(got, want) {
		diffPath := filepath.Join(t.TempDir(), "report.actual.html")

		err := os.WriteFile(diffPath, got, 0o644)
		if err != nil {
			t.Fatalf("write actual: %v", err)
		}

		t.Errorf("HTML output does not match golden file.\n"+
			"  golden: %s (%d bytes)\n"+
			"  actual: %s (%d bytes)\n"+
			"hint: run UPDATE_GOLDEN=1 go test -run TestReport_WriteHTML_GoldenFile to update",
			goldenPath, len(want), diffPath, len(got))
	}
}
