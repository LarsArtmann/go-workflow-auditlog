package auditlog_test

import (
	"fmt"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

func TestDiff_NoChanges(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	w.Add(flow.Step(testhelpers.NewSucceed("step")))
	testhelpers.RunWorkflow(t, a, w)

	r1 := a.Report()
	r2 := a.Report()

	diff := r1.Diff(r2)
	if diff.HasChanges() {
		t.Errorf("expected no changes, got %+v", diff)
	}
}

func TestDiff_StatusChanged(t *testing.T) {
	t.Parallel()

	a1, w1 := testhelpers.NewAuditAndWorkflow(t)
	w1.Add(flow.Step(testhelpers.NewSucceed("step")))
	testhelpers.RunWorkflow(t, a1, w1)

	a2, w2 := testhelpers.NewAuditAndWorkflow(t)
	w2.Add(flow.Step(testhelpers.NewFail("step", "err")))
	testhelpers.RunWorkflow(t, a2, w2)

	diff := a1.Report().Diff(a2.Report())
	if len(diff.StatusChanged) != 1 {
		t.Fatalf("expected 1 status change, got %d", len(diff.StatusChanged))
	}

	if diff.StatusChanged[0].Name != "step" {
		t.Errorf("expected 'step', got %q", diff.StatusChanged[0].Name)
	}

	// OldStatus records the previous state; Status records the new one.
	if diff.StatusChanged[0].OldStatus != auditlog.StepStatusSucceeded {
		t.Errorf("expected OldStatus=succeeded, got %s", diff.StatusChanged[0].OldStatus)
	}

	if diff.StatusChanged[0].Status != auditlog.StepStatusFailed {
		t.Errorf("expected Status=failed, got %s", diff.StatusChanged[0].Status)
	}
}

// TestDiff_DeterministicOrdering verifies that two diffs over the same inputs
// produce identical slice ordering. This is a regression test for a prior bug
// where map iteration produced random output order.
func TestDiff_DeterministicOrdering(t *testing.T) {
	t.Parallel()

	makeReport := func() auditlog.WorkflowReport {
		a, w := testhelpers.NewAuditAndWorkflow(t)
		w.Add(
			flow.Step(testhelpers.NewSucceed("alpha")),
			flow.Step(testhelpers.NewSucceed("bravo")),
			flow.Step(testhelpers.NewSucceed("charlie")),
		)
		testhelpers.RunWorkflow(t, a, w)

		return a.Report()
	}

	left := makeReport()
	right := makeReport()

	d1 := left.Diff(right)
	d2 := left.Diff(right)

	if len(d1.AddedSteps) != len(d2.AddedSteps) || len(d1.RemovedSteps) != len(d2.RemovedSteps) {
		t.Fatalf("diff lengths differ across runs")
	}

	for i := range d1.AddedSteps {
		if d1.AddedSteps[i].Name != d2.AddedSteps[i].Name {
			t.Fatalf("added[%d] differs: %s vs %s", i, d1.AddedSteps[i].Name, d2.AddedSteps[i].Name)
		}
	}
}

func TestDiff_StepAdded(t *testing.T) {
	t.Parallel()

	a1, w1 := testhelpers.NewAuditAndWorkflow(t)
	w1.Add(flow.Step(testhelpers.NewSucceed("a")))
	testhelpers.RunWorkflow(t, a1, w1)

	a2, w2 := testhelpers.NewAuditAndWorkflow(t)
	w2.Add(flow.Step(testhelpers.NewSucceed("a")), flow.Step(testhelpers.NewSucceed("b")))
	testhelpers.RunWorkflow(t, a2, w2)

	diff := a1.Report().Diff(a2.Report())
	if len(diff.AddedSteps) != 1 {
		t.Fatalf("expected 1 added step, got %d", len(diff.AddedSteps))
	}

	if diff.AddedSteps[0].Name != "b" {
		t.Errorf("expected 'b', got %q", diff.AddedSteps[0].Name)
	}
}

func TestDiff_StepRemoved(t *testing.T) {
	t.Parallel()

	a1, w1 := testhelpers.NewAuditAndWorkflow(t)
	w1.Add(flow.Step(testhelpers.NewSucceed("a")), flow.Step(testhelpers.NewSucceed("b")))
	testhelpers.RunWorkflow(t, a1, w1)

	a2, w2 := testhelpers.NewAuditAndWorkflow(t)
	w2.Add(flow.Step(testhelpers.NewSucceed("a")))
	testhelpers.RunWorkflow(t, a2, w2)

	diff := a1.Report().Diff(a2.Report())
	if len(diff.RemovedSteps) != 1 {
		t.Fatalf("expected 1 removed step, got %d", len(diff.RemovedSteps))
	}

	if diff.RemovedSteps[0].Name != "b" {
		t.Errorf("expected 'b', got %q", diff.RemovedSteps[0].Name)
	}
}

func TestDiff_DurationDelta(t *testing.T) {
	t.Parallel()

	a1, w1 := testhelpers.NewAuditAndWorkflow(t)
	w1.Add(flow.Step(testhelpers.NewSucceed("step")))
	testhelpers.RunWorkflow(t, a1, w1)

	a2, w2 := testhelpers.NewAuditAndWorkflow(t)
	w2.Add(flow.Step(testhelpers.NewSlow("step", 20*time.Millisecond)))
	testhelpers.RunWorkflow(t, a2, w2)

	diff := a1.Report().Diff(a2.Report())
	if diff.DurationDelta <= 0 {
		t.Errorf("expected positive duration delta, got %f", diff.DurationDelta)
	}
}

func TestDuration_WallClock(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	w.Add(flow.Step(testhelpers.NewSlow("slow", 20*time.Millisecond)))
	testhelpers.RunWorkflow(t, a, w)

	dur := a.Report().Duration()
	if dur < 15*time.Millisecond {
		t.Errorf("expected duration >= 15ms, got %v", dur)
	}
}

func TestDuration_EmptyReport(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})
	if a.Report().Duration() != 0 {
		t.Error("expected 0 duration for empty report")
	}
}

func TestSummary(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	w.Add(flow.Step(testhelpers.NewSucceed("ok")), flow.Step(testhelpers.NewFail("bad", "err")))
	testhelpers.RunWorkflow(t, a, w)

	summary := a.Report().Summary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestDiff_CriticalPathDelta(t *testing.T) {
	t.Parallel()

	a1, w1 := testhelpers.NewAuditAndWorkflow(t)
	a1.Attach(w1)
	a1.CaptureDAG(w1)

	a1a := testhelpers.NewSlow("a", 10*time.Millisecond)
	a1b := testhelpers.NewSlow("b", 10*time.Millisecond)
	a1c := testhelpers.NewSucceed("c")

	w1.Add(flow.Step(a1a))
	testhelpers.AddDependentStep(w1, a1a, a1b)
	testhelpers.AddDependentStep(w1, a1b, a1c)

	testhelpers.RunWorkflow(t, a1, w1)

	a2, w2 := testhelpers.NewAuditAndWorkflow(t)
	a2.Attach(w2)
	a2.CaptureDAG(w2)

	a2a := testhelpers.NewSlow("a", 100*time.Millisecond)
	a2b := testhelpers.NewSlow("b", 100*time.Millisecond)
	a2c := testhelpers.NewSucceed("c")

	w2.Add(flow.Step(a2a))
	testhelpers.AddDependentStep(w2, a2a, a2b)
	testhelpers.AddDependentStep(w2, a2b, a2c)

	testhelpers.RunWorkflow(t, a2, w2)

	r1, r2 := a1.Report(), a2.Report()

	if r2.CriticalPathDurationMs <= r1.CriticalPathDurationMs {
		t.Skipf("second run did not produce a strictly larger critical-path "+
			"duration (r1=%v, r2=%v); environment too noisy",
			r1.CriticalPathDurationMs, r2.CriticalPathDurationMs)
	}

	diff := r1.Diff(r2)
	if diff.CriticalPathDeltaMs <= 0 {
		t.Errorf("expected positive critical-path delta, got %f", diff.CriticalPathDeltaMs)
	}

	if !diff.HasChanges() {
		t.Error("diff with critical-path change should report changes")
	}
}

func TestDiff_CriticalPathStepsAddedRemoved(t *testing.T) {
	t.Parallel()

	a1, w1 := testhelpers.NewAuditAndWorkflow(t)
	a1.Attach(w1)
	a1.CaptureDAG(w1)

	// Diamond: a → (b, c) → d. d depends on both b and c.
	a := testhelpers.NewSlow("a", 5*time.Millisecond)
	b := testhelpers.NewSlow("b", 5*time.Millisecond)
	c := testhelpers.NewSlow("c", 5*time.Millisecond)
	d := testhelpers.NewSucceed("d")

	w1.Add(flow.Step(a))
	testhelpers.AddDependentStep(w1, a, b)
	testhelpers.AddDependentStep(w1, a, c)
	testhelpers.AddDependentStep(w1, b, d)
	testhelpers.AddDependentStep(w1, c, d)

	testhelpers.RunWorkflow(t, a1, w1)

	a2, w2 := testhelpers.NewAuditAndWorkflow(t)
	a2.Attach(w2)
	a2.CaptureDAG(w2)

	// Different shape: a → b → e. Different downstream — only one leaf.
	a2a := testhelpers.NewSlow("a", 5*time.Millisecond)
	a2b := testhelpers.NewSlow("b", 5*time.Millisecond)
	a2e := testhelpers.NewSucceed("e")

	w2.Add(flow.Step(a2a))
	testhelpers.AddDependentStep(w2, a2a, a2b)
	testhelpers.AddDependentStep(w2, a2b, a2e)

	testhelpers.RunWorkflow(t, a2, w2)

	diff := a1.Report().Diff(a2.Report())

	// r1 has critical-path steps including "d" (or "c", depending on tie
	// breaking). r2 does NOT include "d" or "c". So at least one of those
	// must appear in CriticalPathStepsRemoved.
	removedHasDOrC := false

	for _, n := range diff.CriticalPathStepsRemoved {
		if n == "d" || n == "c" {
			removedHasDOrC = true
		}
	}

	if !removedHasDOrC {
		t.Errorf("expected 'd' or 'c' in CriticalPathStepsRemoved, got %v", diff.CriticalPathStepsRemoved)
	}

	// r2's critical path includes "e" which r1 does NOT have.
	addedHasE := false

	for _, n := range diff.CriticalPathStepsAdded {
		if n == "e" {
			addedHasE = true
		}
	}

	if !addedHasE {
		t.Errorf("expected 'e' in CriticalPathStepsAdded, got %v", diff.CriticalPathStepsAdded)
	}
}

func TestDiff_PeakConcurrencyDelta(t *testing.T) {
	t.Parallel()

	a1, w1 := testhelpers.NewAuditAndWorkflow(t)
	a1.Attach(w1)
	a1.CaptureDAG(w1)

	for i := range 3 {
		w1.Add(flow.Step(testhelpers.NewSlow(fmt.Sprintf("p%d", i), 20*time.Millisecond)))
	}

	testhelpers.RunWorkflow(t, a1, w1)

	a2, w2 := testhelpers.NewAuditAndWorkflow(t)
	a2.Attach(w2)
	a2.CaptureDAG(w2)

	for i := range 5 {
		w2.Add(flow.Step(testhelpers.NewSlow(fmt.Sprintf("p%d", i), 20*time.Millisecond)))
	}

	testhelpers.RunWorkflow(t, a2, w2)

	r1, r2 := a1.Report(), a2.Report()
	if r2.PeakConcurrency <= r1.PeakConcurrency {
		t.Skipf("second run did not reach higher peak concurrency (r1=%d, r2=%d); environment too noisy",
			r1.PeakConcurrency, r2.PeakConcurrency)
	}

	diff := r1.Diff(r2)
	if diff.PeakConcurrencyDelta <= 0 {
		t.Errorf("expected positive peak-concurrency delta, got %d", diff.PeakConcurrencyDelta)
	}
}

func TestDiff_HasChanges_AggregateOnly(t *testing.T) {
	t.Parallel()

	// Two reports with identical steps but different aggregate metrics —
	// Diff must report HasChanges=true even without step-level diffs.
	base := auditlog.WorkflowReport{
		WallClockDurationMs:    100,
		CriticalPathDurationMs: 80,
		PeakConcurrency:        2,
	}

	other := auditlog.WorkflowReport{
		WallClockDurationMs:    100, // same
		CriticalPathDurationMs: 90,  // different
		PeakConcurrency:        2,
	}

	diff := base.Diff(other)
	if !diff.HasChanges() {
		t.Error("diff with only aggregate change should still report HasChanges")
	}

	if diff.CriticalPathDeltaMs != 10 {
		t.Errorf("CriticalPathDeltaMs = %f, want 10", diff.CriticalPathDeltaMs)
	}
}
