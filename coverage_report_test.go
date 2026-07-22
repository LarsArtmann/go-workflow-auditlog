package auditlog_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

// TestCoverage_AllEventTypes covers the core enumerator so visualization code
// is not required to exercise it.
func TestCoverage_AllEventTypes(t *testing.T) {
	t.Parallel()

	got := auditlog.AllEventTypes()
	if len(got) != 2 {
		t.Fatalf("expected 2 event types, got %d", len(got))
	}

	if got[0] != auditlog.EventTypeAttemptStart || got[1] != auditlog.EventTypeAttemptEnd {
		t.Errorf("unexpected event types: %v", got)
	}
}

// TestCoverage_AllStepStatuses covers the core enumerator so visualization code
// is not required to exercise it.
func TestCoverage_AllStepStatuses(t *testing.T) {
	t.Parallel()

	got := auditlog.AllStepStatuses()
	want := []auditlog.StepStatus{
		auditlog.StepStatusPending,
		auditlog.StepStatusRunning,
		auditlog.StepStatusSucceeded,
		auditlog.StepStatusFailed,
		auditlog.StepStatusCanceled,
		auditlog.StepStatusSkipped,
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d statuses, got %d", len(want), len(got))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("status %d: expected %s, got %s", i, want[i], got[i])
		}
	}
}

// TestCoverage_Report_SkippedSteps covers WorkflowReport.SkippedSteps.
func TestCoverage_Report_SkippedSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	upstream := testhelpers.NewFail("failing", "fail")
	downstream := testhelpers.NewSucceed("skipped")
	testhelpers.AddDependentStep(w, upstream, downstream)
	testhelpers.RunWorkflow(t, a, w)

	skipped := a.Report().SkippedSteps()
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped step, got %d", len(skipped))
	}

	if skipped[0].Name != "skipped" {
		t.Errorf("expected 'skipped', got %q", skipped[0].Name)
	}
}

// TestCoverage_Report_CriticalPath covers WorkflowReport.CriticalPath and the
// underlying computeCriticalPathDuration logic.
func TestCoverage_Report_CriticalPath(t *testing.T) {
	t.Parallel()

	ms := 1.0

	report := auditlog.WorkflowReport{
		WorkflowID: "critical-path-test",
		Steps: []auditlog.StepInfo{
			{
				StepRef:    auditlog.StepRef{Name: "a"},
				Status:     auditlog.StepStatusSucceeded,
				DurationMs: &ms,
			},
			{
				StepRef:    auditlog.StepRef{Name: "b"},
				Status:     auditlog.StepStatusSucceeded,
				DurationMs: &ms,
				Dependencies: []auditlog.StepRef{
					{Name: "a"},
				},
			},
			{
				StepRef:    auditlog.StepRef{Name: "c"},
				Status:     auditlog.StepStatusSucceeded,
				DurationMs: &ms,
				Dependencies: []auditlog.StepRef{
					{Name: "b"},
				},
			},
		},
		EventCount: 0,
		StepCount:  3,
	}

	path := report.CriticalPath()
	if len(path) != 3 {
		t.Fatalf("expected 3 steps on critical path, got %d", len(path))
	}

	if path[0].Name != "a" || path[1].Name != "b" || path[2].Name != "c" {
		t.Errorf("unexpected critical path: %v", namesFromSteps(path))
	}

	if report.CriticalPathDurationMs != 0 {
		t.Errorf("expected zero duration before finalize, got %f", report.CriticalPathDurationMs)
	}

	empty := auditlog.WorkflowReport{WorkflowID: "empty"}
	if empty.CriticalPath() != nil {
		t.Error("expected nil CriticalPath for empty report")
	}
}

// TestCoverage_Report_PeakConcurrencySteps covers
// WorkflowReport.PeakConcurrencySteps and the underlying
// computePeakConcurrencySteps logic.
func TestCoverage_Report_PeakConcurrencySteps(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	report := auditlog.WorkflowReport{
		WorkflowID: "concurrency-test",
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "a"}, Status: auditlog.StepStatusSucceeded},
			{StepRef: auditlog.StepRef{Name: "b"}, Status: auditlog.StepStatusSucceeded},
		},
		Events: []auditlog.Event{
			{
				EventType: auditlog.EventTypeAttemptStart,
				StepRef:   auditlog.StepRef{Name: "a"},
				Timestamp: now,
			},
			{
				EventType: auditlog.EventTypeAttemptStart,
				StepRef:   auditlog.StepRef{Name: "b"},
				Timestamp: now.Add(time.Millisecond),
			},
			{
				EventType: auditlog.EventTypeAttemptEnd,
				StepRef:   auditlog.StepRef{Name: "a"},
				Timestamp: now.Add(2 * time.Millisecond),
			},
			{
				EventType: auditlog.EventTypeAttemptEnd,
				StepRef:   auditlog.StepRef{Name: "b"},
				Timestamp: now.Add(3 * time.Millisecond),
			},
		},
		EventCount: 4,
		StepCount:  2,
	}

	peakSteps := report.PeakConcurrencySteps()
	if len(peakSteps) != 2 {
		t.Fatalf("expected 2 peak-concurrency steps, got %d", len(peakSteps))
	}

	if peakSteps[0].Name != "a" || peakSteps[1].Name != "b" {
		t.Errorf("unexpected peak steps: %v", namesFromSteps(peakSteps))
	}

	empty := auditlog.WorkflowReport{WorkflowID: "empty"}
	if empty.PeakConcurrencySteps() != nil {
		t.Error("expected nil PeakConcurrencySteps for empty report")
	}
}

// TestCoverage_Report_ExportJSONAndNDJSON covers the core file-export methods
// on WorkflowReport.
func TestCoverage_Report_ExportJSONAndNDJSON(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewSucceed("export-step")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "report.json")
	ndjsonPath := filepath.Join(dir, "events.ndjson")

	report := a.Report()

	err := report.ExportJSON(jsonPath)
	if err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}

	err = report.ExportNDJSON(ndjsonPath)
	if err != nil {
		t.Fatalf("ExportNDJSON error: %v", err)
	}

	_, err = os.Stat(jsonPath)
	if err != nil {
		t.Errorf("JSON file not written: %v", err)
	}

	_, err = os.Stat(ndjsonPath)
	if err != nil {
		t.Errorf("NDJSON file not written: %v", err)
	}
}

// TestCoverage_Report_Summary covers WorkflowReport.Summary failure paths.
func TestCoverage_Report_Summary(t *testing.T) {
	t.Parallel()

	failed := auditlog.WorkflowReport{
		WorkflowID:          "wf",
		StepCount:           1,
		SucceededCount:      0,
		FailedCount:         1,
		SkippedCount:        0,
		WorkflowSucceeded:   false,
		WallClockDurationMs: 1.5,
	}

	if summary := failed.Summary(); summary == "" {
		t.Error("expected non-empty failure summary")
	}

	failedWithReason := failed

	failedWithReason.FailureReason = "explosion"
	if summary := failedWithReason.Summary(); summary == "" {
		t.Error("expected non-empty failure summary with reason")
	}

	pending := auditlog.WorkflowReport{
		WorkflowID:          "wf",
		StepCount:           1,
		SucceededCount:      0,
		FailedCount:         0,
		SkippedCount:        0,
		PendingCount:        1,
		RunningCount:        0,
		WorkflowSucceeded:   false,
		WallClockDurationMs: 0,
	}

	if summary := pending.Summary(); summary == "" {
		t.Error("expected non-empty pending summary")
	}
}

// TestCoverage_Report_WriteJSON_ErrorPath covers the error branch of
// WorkflowReport.WriteJSON when given a failing writer.
func TestCoverage_Report_WriteJSON_ErrorPath(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewSucceed("json-error")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	writer := &failingWriter{}

	err := report.WriteJSON(writer)
	if err == nil {
		t.Fatal("expected error from failing writer")
	}

	if !errors.Is(err, auditlog.ErrRenderFailed) {
		t.Errorf("expected error to wrap ErrRenderFailed, got: %v", err)
	}
}

// TestCoverage_StepStatus_ColorUnknown covers the unknown-status branch of
// StepStatus.Color.
func TestCoverage_StepStatus_ColorUnknown(t *testing.T) {
	t.Parallel()

	unknown := auditlog.StepStatus("unknown-status")
	fill, font := unknown.Color()

	if fill != "" || font != "" {
		t.Errorf("expected empty colors, got fill=%q font=%q", fill, font)
	}
}

func namesFromSteps(steps []auditlog.StepInfo) []string {
	names := make([]string, 0, len(steps))

	for _, s := range steps {
		names = append(names, s.Name)
	}

	return names
}

type failingWriter struct{}

func (f *failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}
