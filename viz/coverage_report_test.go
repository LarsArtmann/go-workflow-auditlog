package viz_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	output "github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

// --- Report query method tests ---

func TestReport_StepByName(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewSucceed("find-me")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	step := report.StepByName("find-me")
	if step == nil {
		t.Fatal("expected to find step 'find-me'")
	}

	if step.Name != "find-me" {
		t.Errorf("expected name 'find-me', got %q", step.Name)
	}

	if report.StepByName("nonexistent") != nil {
		t.Error("expected nil for nonexistent step")
	}
}

func TestReport_EventsByType(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewSucceed("typed-step")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	starts := report.EventsByType(auditlog.EventTypeAttemptStart)
	if len(starts) < 1 {
		t.Errorf("expected at least 1 start event, got %d", len(starts))
	}

	ends := report.EventsByType(auditlog.EventTypeAttemptEnd)
	if len(ends) < 1 {
		t.Errorf("expected at least 1 end event, got %d", len(ends))
	}
}

func TestReport_SucceededSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	ok := testhelpers.NewSucceed("ok1")
	ok2 := testhelpers.NewSucceed("ok2")
	bad := testhelpers.NewFail("bad", "err")
	w.Add(
		flow.Step(ok),
		flow.Step(ok2),
		flow.Step(bad),
	)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	succeeded := report.SucceededSteps()
	if len(succeeded) != 2 {
		t.Fatalf("expected 2 succeeded steps, got %d", len(succeeded))
	}
}

func TestReport_SkippedSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	upstream := testhelpers.NewFail("failing", "fail")
	downstream := testhelpers.NewSucceed("skipped")
	testhelpers.AddDependentStep(w, upstream, downstream)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	skipped := report.SkippedSteps()
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped step, got %d", len(skipped))
	}

	if skipped[0].Name != "skipped" {
		t.Errorf("expected 'skipped', got %q", skipped[0].Name)
	}
}

// --- Validate error path tests ---

func TestReport_Validate_EventCountMismatch(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		EventCount: 99,
		Events:     []auditlog.Event{{}},
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("expected validation error for event count mismatch")
	}

	if !errors.Is(err, auditlog.ErrEventCountMismatch) {
		t.Errorf("expected error to wrap auditlog.ErrEventCountMismatch, got: %v", err)
	}
}

func TestReport_Validate_StepCountMismatch(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		StepCount: 99,
		Steps:     []auditlog.StepInfo{{}},
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("expected validation error for step count mismatch")
	}

	if !errors.Is(err, auditlog.ErrStepCountMismatch) {
		t.Errorf("expected error to wrap auditlog.ErrStepCountMismatch, got: %v", err)
	}
}

// TestReport_Validate_StatusDrift confirms the status drift check actually
// fires when a step's stored Status disagrees with its Error pointer. This
// is a regression test for a prior bug where the check could never fail.
func TestReport_Validate_StatusDrift(t *testing.T) {
	t.Parallel()

	errMsg := "boom"

	report := auditlog.WorkflowReport{
		EventCount: 1,
		Events:     []auditlog.Event{{}},
		StepCount:  1,
		Steps: []auditlog.StepInfo{{
			Status: auditlog.StepStatusPending, // non-terminal, but Error implies failure
			Error:  &errMsg,
		}},
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("expected validation error for status drift")
	}

	if !errors.Is(err, auditlog.ErrStatusDrift) {
		t.Errorf("expected error to wrap auditlog.ErrStatusDrift, got: %v", err)
	}

	testhelpers.AssertContains(t, err.Error(), "does not match derived status",
		"expected error mentioning status mismatch")
}

func TestReport_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s1 := testhelpers.NewSucceed("rt-1")
	s2 := testhelpers.NewFail("rt-2", "fail")
	testhelpers.AddDependentStep(w, s1, s2)
	testhelpers.RunWorkflow(t, a, w)

	var buf bytes.Buffer

	err := a.WriteJSON(&buf)
	if err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	var report auditlog.WorkflowReport

	err = json.Unmarshal(buf.Bytes(), &report)
	if err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	// Verify the round-tripped report has correct data.
	if report.Version != auditlog.SchemaVersion {
		t.Errorf("version mismatch: %s", report.Version)
	}

	testhelpers.AssertStepCount(t, report, 2)
}

func TestReport_WithRetryTiming(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewFlaky("retry-timing", 2)
	w.Add(
		flow.Step(s).Retry(testhelpers.RetryOpts(5)),
	)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	step := report.StepByName("retry-timing")

	// Each attempt should have a start/end event pair.
	startEvents := report.EventsByType(auditlog.EventTypeAttemptStart)
	endEvents := report.EventsByType(auditlog.EventTypeAttemptEnd)

	if len(startEvents) != step.AttemptCount {
		t.Errorf("expected %d start events, got %d", step.AttemptCount, len(startEvents))
	}

	if len(endEvents) != step.AttemptCount {
		t.Errorf("expected %d end events, got %d", step.AttemptCount, len(endEvents))
	}

	// Verify attempt numbers are sequential.
	for i, evt := range startEvents {
		if evt.Attempt != i+1 {
			t.Errorf("start event %d: expected attempt %d, got %d", i, i+1, evt.Attempt)
		}
	}
}

func TestReport_EmptyWorkflow(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})
	report := a.Report()

	testhelpers.AssertStepCount(t, report, 0)

	testhelpers.AssertEventCount(t, report, 0)

	if !report.WorkflowSucceeded {
		t.Error("empty workflow should be considered succeeded")
	}
}

// TestWorkflowSucceeded_PendingStepsIsFalse verifies that WorkflowSucceeded
// is false when a report contains non-terminal steps. The aggregate is
// recomputed through Filtered() → buildReportFromCore → finalizeDenormalized,
// which previously returned true as long as no step had failed or canceled.
func TestWorkflowSucceeded_PendingStepsIsFalse(t *testing.T) {
	t.Parallel()

	// Construct a report with a non-terminal step. WorkflowSucceeded is the
	// zero value here, so we route it through Filtered() to recompute.
	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			testhelpers.StepFixture("stuck", auditlog.StepStatusRunning),
		},
	}

	recomputed := raw.Filtered()

	if recomputed.WorkflowSucceeded {
		t.Error("expected WorkflowSucceeded=false when steps are still running")
	}
}

func TestReport_AggregateCounts_PendingRunning(t *testing.T) {
	t.Parallel()

	// Build a report with a non-terminal step directly.
	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			testhelpers.StepFixture("pending-step", auditlog.StepStatusPending),
			testhelpers.StepFixture("running-step", auditlog.StepStatusRunning),
			testhelpers.StepFixture("done-step", auditlog.StepStatusSucceeded),
		},
	}

	recomputed := raw.Filtered()

	testhelpers.AssertCount(t, "PendingCount", recomputed.PendingCount, 1)
	testhelpers.AssertCount(t, "RunningCount", recomputed.RunningCount, 1)
	testhelpers.AssertCount(t, "SucceededCount", recomputed.SucceededCount, 1)
}

func TestReport_PeakConcurrency_ParallelSteps(t *testing.T) {
	t.Parallel()

	// Use slow steps to ensure true overlap in the event stream.
	a, w := testhelpers.NewAuditAndWorkflow(t)
	testhelpers.AddSlowParallelSteps(w, 20*time.Millisecond)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	// Two parallel slow steps should produce a peak concurrency of 2.
	testhelpers.AssertPeakConcurrency(t, report, 2)
}

func TestReport_PeakConcurrency_SequentialSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	testhelpers.AddDependentStep(w, testhelpers.NewSucceed("parent"), testhelpers.NewSucceed("child"))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	// Sequential steps should never overlap.
	testhelpers.AssertPeakConcurrency(t, report, 1)
}

func TestReport_CriticalPathDuration_DependentChain(t *testing.T) {
	t.Parallel()

	// Construct a report with a known dependency chain.
	// parent (10ms) -> child (20ms) -> grandchild (30ms)
	// plus an independent leaf (5ms)
	// critical path = 10 + 20 + 30 = 60ms
	d10 := 10.0
	d20 := 20.0
	d30 := 30.0
	d5 := 5.0

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{
				StepRef:    auditlog.StepRef{Name: "parent"},
				Status:     auditlog.StepStatusSucceeded,
				DurationMs: &d10,
			},
			{
				StepRef:      auditlog.StepRef{Name: "child"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &d20,
				Dependencies: []auditlog.StepRef{{Name: "parent"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "grandchild"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &d30,
				Dependencies: []auditlog.StepRef{{Name: "child"}},
			},
			{
				StepRef:    auditlog.StepRef{Name: "leaf"},
				Status:     auditlog.StepStatusSucceeded,
				DurationMs: &d5,
			},
		},
	}

	recomputed := raw.Filtered()

	want := 60.0
	if recomputed.CriticalPathDurationMs != want {
		t.Errorf("expected CriticalPathDurationMs=%f, got %f", want, recomputed.CriticalPathDurationMs)
	}
}

func TestReport_CriticalPathDuration_Empty(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{Steps: []auditlog.StepInfo{}}
	recomputed := raw.Filtered()

	if recomputed.CriticalPathDurationMs != 0 {
		t.Errorf("expected CriticalPathDurationMs=0 for empty report, got %f", recomputed.CriticalPathDurationMs)
	}
}

// TestReport_PeakConcurrency_HighFanOut verifies peak concurrency with 8+
// independent slow steps. The existing test only covers 2 parallel steps; this
// stress test ensures the event-stream scan correctly counts higher fan-out
// (the report flagged "add a higher-fan-out case").
func TestReport_PeakConcurrency_HighFanOut(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	// Wire 8 independent slow steps so they all overlap in time.
	for i := range 8 {
		w.Add(flow.Step(testhelpers.NewSlow(fmt.Sprintf("parallel-%d", i), 50*time.Millisecond)))
	}

	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	// With 8 independent 50ms steps, at least 4 must overlap on any machine
	// fast enough to schedule goroutines within the step duration. Asserting
	// >= 4 (rather than exactly 8) avoids CI-scheduling flakiness while still
	// proving the scan captures high fan-out (the 2-step test asserts 2).
	if report.PeakConcurrency < 4 {
		t.Errorf("expected PeakConcurrency >= 4 for 8 parallel 50ms steps, got %d", report.PeakConcurrency)
	}
}

// TestReport_CriticalPathDuration_DiamondDAG verifies the memoized DFS
// critical-path computation on a non-linear topology:
//
//	root (10ms) → left (30ms) → bottom (5ms)
//	     \→ right (5ms) ↗
//
// Critical path is root → left → bottom = 45ms (not root → right → bottom = 20ms).
// The report flagged "CriticalPath test with diamond DAG" as missing.
func TestReport_CriticalPathDuration_DiamondDAG(t *testing.T) {
	t.Parallel()

	rootD := 10.0
	leftD := 30.0
	rightD := 5.0
	bottomD := 5.0

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{
				StepRef:    auditlog.StepRef{Name: "root"},
				Status:     auditlog.StepStatusSucceeded,
				DurationMs: &rootD,
			},
			{
				StepRef:      auditlog.StepRef{Name: "left"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &leftD,
				Dependencies: []auditlog.StepRef{{Name: "root"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "right"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &rightD,
				Dependencies: []auditlog.StepRef{{Name: "root"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "bottom"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &bottomD,
				Dependencies: []auditlog.StepRef{{Name: "left"}, {Name: "right"}},
			},
		},
	}

	recomputed := raw.Filtered()

	// Critical path: root(10) → left(30) → bottom(5) = 45ms.
	// NOT root(10) → right(5) → bottom(5) = 20ms.
	want := 45.0
	if recomputed.CriticalPathDurationMs != want {
		t.Errorf("diamond DAG: expected CriticalPathDurationMs=%f (root→left→bottom), got %f",
			want, recomputed.CriticalPathDurationMs)
	}
}

// TestReport_CriticalPath_DependentChain verifies the returned step chain
// matches the longest dependency path.
func TestReport_CriticalPath_DependentChain(t *testing.T) {
	t.Parallel()

	d10 := 10.0
	d20 := 20.0
	d30 := 30.0
	d5 := 5.0

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "parent"}, Status: auditlog.StepStatusSucceeded, DurationMs: &d10},
			{
				StepRef:      auditlog.StepRef{Name: "child"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &d20,
				Dependencies: []auditlog.StepRef{{Name: "parent"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "grandchild"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &d30,
				Dependencies: []auditlog.StepRef{{Name: "child"}},
			},
			{StepRef: auditlog.StepRef{Name: "leaf"}, Status: auditlog.StepStatusSucceeded, DurationMs: &d5},
		},
	}

	path := raw.CriticalPath()

	if len(path) != 3 {
		t.Fatalf("expected 3 steps in critical path, got %d", len(path))
	}

	want := []string{"parent", "child", "grandchild"}

	for i, step := range path {
		if step.Name != want[i] {
			t.Errorf("path[%d]: expected %q, got %q", i, want[i], step.Name)
		}
	}
}

// TestReport_CriticalPath_Empty verifies nil return for empty reports.
func TestReport_CriticalPath_Empty(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{Steps: []auditlog.StepInfo{}}

	if path := report.CriticalPath(); path != nil {
		t.Errorf("expected nil critical path for empty report, got %v", path)
	}
}

// TestReport_CriticalPath_DiamondDAG verifies the critical path selects the
// longer branch in a diamond topology: root → left → bottom (not root → right → bottom).
func TestReport_CriticalPath_DiamondDAG(t *testing.T) {
	t.Parallel()

	rootD := 10.0
	leftD := 30.0
	rightD := 5.0
	bottomD := 5.0

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "root"}, Status: auditlog.StepStatusSucceeded, DurationMs: &rootD},
			{
				StepRef:      auditlog.StepRef{Name: "left"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &leftD,
				Dependencies: []auditlog.StepRef{{Name: "root"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "right"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &rightD,
				Dependencies: []auditlog.StepRef{{Name: "root"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "bottom"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &bottomD,
				Dependencies: []auditlog.StepRef{{Name: "left"}, {Name: "right"}},
			},
		},
	}

	path := raw.CriticalPath()

	want := []string{"root", "left", "bottom"}

	if len(path) != len(want) {
		t.Fatalf("expected %d steps, got %d: %v", len(want), len(path), path)
	}

	for i, step := range path {
		if step.Name != want[i] {
			t.Errorf("path[%d]: expected %q, got %q", i, want[i], step.Name)
		}
	}
}

// TestReport_PeakConcurrencySteps_ParallelSteps verifies that steps active at
// the peak concurrency moment are returned.
func TestReport_PeakConcurrencySteps_ParallelSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	testhelpers.AddSlowParallelSteps(w, 20*time.Millisecond)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	steps := report.PeakConcurrencySteps()

	if len(steps) != 2 {
		t.Fatalf("expected 2 steps at peak, got %d", len(steps))
	}
}

// TestReport_PeakConcurrencySteps_Empty verifies nil for reports with no events.
func TestReport_PeakConcurrencySteps_Empty(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{}

	if steps := report.PeakConcurrencySteps(); steps != nil {
		t.Errorf("expected nil for empty report, got %v", steps)
	}
}

func TestReport_FailureReason_FailedSteps(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			testhelpers.StepFixture("ok", auditlog.StepStatusSucceeded),
			testhelpers.StepFixture("bad-a", auditlog.StepStatusFailed),
			testhelpers.StepFixture("bad-b", auditlog.StepStatusFailed),
		},
	}

	recomputed := raw.Filtered()

	if recomputed.WorkflowSucceeded {
		t.Error("expected workflow to be failed")
	}

	testhelpers.AssertFailureReason(t, recomputed, "2 step(s) failed: bad-a, bad-b")
}

func TestReport_FailureReason_CanceledSteps(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			testhelpers.StepFixture("ok", auditlog.StepStatusSucceeded),
			testhelpers.StepFixture("cancel", auditlog.StepStatusCanceled),
		},
	}

	recomputed := raw.Filtered()

	testhelpers.AssertFailureReason(t, recomputed, "1 step(s) canceled: cancel")
}

func TestReport_FailureReason_Success(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			testhelpers.StepFixture("ok", auditlog.StepStatusSucceeded),
		},
	}

	recomputed := raw.Filtered()

	if !recomputed.WorkflowSucceeded {
		t.Error("expected workflow to be succeeded")
	}

	if recomputed.FailureReason != "" {
		t.Errorf("expected empty FailureReason for success, got %q", recomputed.FailureReason)
	}
}

func TestReport_WallClockDuration_ParallelLessThanSum(t *testing.T) {
	t.Parallel()

	// Two parallel slow steps: wall-clock ≈ max(20ms, 20ms) = 20ms,
	// but TotalDurationMs = 20 + 20 = 40ms. Wall-clock should be less.
	a, w := testhelpers.NewAuditAndWorkflow(t)
	testhelpers.AddSlowParallelSteps(w, 20*time.Millisecond)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	if report.WallClockDurationMs >= report.TotalDurationMs {
		t.Errorf("wall-clock (%.1fms) should be less than sum (%.1fms) for parallel steps",
			report.WallClockDurationMs, report.TotalDurationMs)
	}

	// Duration() method should agree with WallClockDurationMs field.
	methodMs := float64(report.Duration().Microseconds()) / 1000.0
	if absDiff(report.WallClockDurationMs, methodMs) > 1.0 {
		t.Errorf("Duration() method (%.1fms) disagrees with WallClockDurationMs field (%.1fms)",
			methodMs, report.WallClockDurationMs)
	}
}

func TestReport_WallClockDuration_EmptyReport(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{}
	recomputed := raw.Filtered()

	if recomputed.WallClockDurationMs != 0 {
		t.Errorf("expected 0 wall-clock for empty report, got %f", recomputed.WallClockDurationMs)
	}
}

func TestReport_Summary_WithFailureReason(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{
		WorkflowID: "test-wf",
		Steps: []auditlog.StepInfo{
			testhelpers.StepFixture("ok", auditlog.StepStatusSucceeded),
			testhelpers.StepFixture("bad", auditlog.StepStatusFailed),
		},
	}

	recomputed := raw.Filtered()

	summary := recomputed.Summary()
	testhelpers.AssertContains(t, summary, "bad", "summary should contain failed step name")
	testhelpers.AssertContains(t, summary, "failed", "summary should mention failure")
}

func TestReport_Summary_SuccessNoReason(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{
		WorkflowID: "test-wf",
		Steps: []auditlog.StepInfo{
			testhelpers.StepFixture("ok", auditlog.StepStatusSucceeded),
		},
	}

	recomputed := raw.Filtered()

	summary := recomputed.Summary()
	if strings.Contains(summary, "—") {
		t.Errorf("successful summary should not contain failure reason, got %q", summary)
	}
}

func TestReport_Validate_CountMismatch(t *testing.T) {
	t.Parallel()

	// Construct a report where SucceededCount is wrong.
	raw := auditlog.WorkflowReport{
		SucceededCount: 2, // lie — only 1 succeeded step
		StepCount:      1,
		Steps: []auditlog.StepInfo{
			testhelpers.StepFixture("ok", auditlog.StepStatusSucceeded),
		},
	}

	err := raw.Validate()
	if err == nil {
		t.Fatal("expected validation error for count mismatch")
	}

	if !errors.Is(err, auditlog.ErrCountMismatch) {
		t.Errorf("expected ErrCountMismatch, got %v", err)
	}
}

func TestReport_Validate_CountsMatch(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	testhelpers.AddParallelSteps(w, testhelpers.NewSucceed("a"), testhelpers.NewFail("b", "boom"))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	err := report.Validate()
	if err != nil {
		t.Errorf("expected valid report, got %v", err)
	}
}

func TestDiff_DurationDelta_UsesWallClock(t *testing.T) {
	t.Parallel()

	// Two reports with identical wall-clock but different summed durations.
	// DurationDelta should be ~0 (wall-clock), not the sum difference.
	base := auditlog.WorkflowReport{
		WallClockDurationMs: 100.0,
		TotalDurationMs:     150.0,
	}
	other := auditlog.WorkflowReport{
		WallClockDurationMs: 102.0,
		TotalDurationMs:     200.0,
	}

	result := base.Diff(other)

	wantDelta := 2.0 // 102 - 100
	if absDiff(result.DurationDelta, wantDelta) > 0.01 {
		t.Errorf("expected DurationDelta=%.1f (wall-clock), got %.1f",
			wantDelta, result.DurationDelta)
	}
}

// absDiff returns the absolute difference between two floats.
func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}

	return b - a
}

// TestCoverage_Report_ExportTable covers WorkflowReport.ExportTable (was 0%).
// The Auditor.ExportTable path is tested in output_test.go, but the
// WorkflowReport-level method — used by replayed/loaded reports — was never
// called directly.
func TestCoverage_Report_ExportTable(t *testing.T) {
	t.Parallel()

	report := minimalReport()
	path := filepath.Join(t.TempDir(), "table.csv")

	err := viz.ExportTable(report, path, output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("ExportTable error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty table output file")
	}
}

// TestExportTable_FromReplayedReport verifies the full round-trip pipeline:
// run workflow → NDJSON export → ReadEvents → ReplayEvents → ExportTable.
// This confirms the primary offline-analysis use case for WorkflowReport.Export*.
func TestExportTable_FromReplayedReport(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	w.Add(flow.Step(testhelpers.NewSucceed("replay-table-step")))
	testhelpers.RunWorkflow(t, a, w)

	var ndjsonBuf bytes.Buffer

	err := a.WriteNDJSON(&ndjsonBuf)
	if err != nil {
		t.Fatalf("WriteNDJSON error: %v", err)
	}

	events, err := auditlog.ReadEvents(&ndjsonBuf)
	if err != nil {
		t.Fatalf("ReadEvents error: %v", err)
	}

	replayed, err := auditlog.ReplayEvents(events)
	if err != nil {
		t.Fatalf("ReplayEvents error: %v", err)
	}

	path := filepath.Join(t.TempDir(), "replayed.csv")

	err = viz.ExportTable(replayed, path, output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("ExportTable from replayed report error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if !strings.Contains(string(data), "replay-table-step") {
		t.Errorf("expected table output to contain step name, got:\n%s", string(data))
	}
}

// TestCoverage_Summary_AllBranches covers all three Summary() return paths:
// success, failure-with-reason, and failure-without-reason.
func TestCoverage_Summary_AllBranches(t *testing.T) {
	t.Parallel()

	// Success branch — WorkflowSucceeded=true.
	successReport := auditlog.WorkflowReport{
		WorkflowID: "wf", StepCount: 1, SucceededCount: 1,
		WorkflowSucceeded: true,
	}
	if !strings.Contains(successReport.Summary(), "wf: 1 steps") {
		t.Errorf("success summary unexpected: %s", successReport.Summary())
	}

	// Failure with explicit reason.
	failedReport := auditlog.WorkflowReport{
		WorkflowID: "wf", StepCount: 2, FailedCount: 1,
		FailureReason: "1 step(s) failed: bad",
	}
	if !strings.Contains(failedReport.Summary(), "1 step(s) failed: bad") {
		t.Errorf("failure-with-reason summary unexpected: %s", failedReport.Summary())
	}

	// Failure without explicit reason (pending steps, no failure reason set).
	pendingReport := auditlog.WorkflowReport{
		WorkflowID: "wf", StepCount: 1, PendingCount: 1,
	}
	s := pendingReport.Summary()

	if !strings.Contains(s, "failed") && !strings.Contains(s, "pending") {
		t.Errorf("failure-without-reason summary unexpected: %s", s)
	}
}

// TestCoverage_MatchEvent_TimeFilters covers the timeFrom/timeTo branches of
// matchEvent (was 75%) via the public Filtered API with time-based options.
func TestCoverage_MatchEvent_TimeFilters(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	dur := 10.0

	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "s"}, Status: auditlog.StepStatusSucceeded},
		},
		EventCount: 2,
		Events: []auditlog.Event{
			{
				StepRef:   auditlog.StepRef{Name: "s"},
				Sequence:  1,
				Timestamp: base,
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
			},
			{
				StepRef:    auditlog.StepRef{Name: "s"},
				Sequence:   2,
				Timestamp:  base.Add(5 * time.Second),
				EventType:  auditlog.EventTypeAttemptEnd,
				Phase:      auditlog.PhaseAfter,
				DurationMs: &dur,
				Status:     auditlog.StepStatusSucceeded,
			},
		},
	}

	from := base.Add(2 * time.Second)
	to := base.Add(10 * time.Second)

	filtered := report.Filtered(
		auditlog.WithTimeRange(from, to),
	)

	if len(filtered.Events) != 1 {
		t.Errorf("expected 1 event in time window, got %d", len(filtered.Events))
	}

	if filtered.Events[0].EventType != auditlog.EventTypeAttemptEnd {
		t.Errorf("expected attempt_end event, got %s", filtered.Events[0].EventType)
	}
}

// TestFilter_CombinedEventTypeAndTimeRange verifies that WithEventsByType and
// WithTimeRange interact correctly when applied together: only events matching
// BOTH the type filter AND the time window should survive.
func TestFilter_CombinedEventTypeAndTimeRange(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "s"}, Status: auditlog.StepStatusSucceeded},
		},
		EventCount: 4,
		Events: []auditlog.Event{
			{
				StepRef:   auditlog.StepRef{Name: "s"},
				Sequence:  1,
				Timestamp: base,
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
			},
			{
				StepRef:   auditlog.StepRef{Name: "s"},
				Sequence:  2,
				Timestamp: base.Add(3 * time.Second),
				EventType: auditlog.EventTypeAttemptEnd,
				Phase:     auditlog.PhaseAfter,
			},
			{
				StepRef:   auditlog.StepRef{Name: "s"},
				Sequence:  3,
				Timestamp: base.Add(6 * time.Second),
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
			},
			{
				StepRef:   auditlog.StepRef{Name: "s"},
				Sequence:  4,
				Timestamp: base.Add(9 * time.Second),
				EventType: auditlog.EventTypeAttemptEnd,
				Phase:     auditlog.PhaseAfter,
			},
		},
	}

	from := base.Add(2 * time.Second)
	to := base.Add(7 * time.Second)

	filtered := report.Filtered(
		auditlog.WithEventsByType(auditlog.EventTypeAttemptStart),
		auditlog.WithTimeRange(from, to),
	)

	// Only the start event at +6s is both an AttemptStart AND within [2s, 7s].
	if len(filtered.Events) != 1 {
		t.Fatalf("expected 1 event matching type+time, got %d", len(filtered.Events))
	}

	evt := filtered.Events[0]
	if evt.EventType != auditlog.EventTypeAttemptStart {
		t.Errorf("expected attempt_start, got %s", evt.EventType)
	}

	if evt.Sequence != 3 {
		t.Errorf("expected sequence 3 (the +6s start), got %d", evt.Sequence)
	}
}
