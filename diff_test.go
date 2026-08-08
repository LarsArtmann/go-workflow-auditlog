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

	// Diff() reads the precomputed CriticalPathDurationMs field directly, so
	// synthetic reports exercise the arithmetic deterministically without
	// relying on wall-clock execution timing (which is noisy under -race or
	// loaded CI and previously forced a t.Skip).
	base := auditlog.WorkflowReport{CriticalPathDurationMs: 500}
	other := auditlog.WorkflowReport{CriticalPathDurationMs: 800}

	diff := base.Diff(other)

	if diff.CriticalPathDeltaMs != 300 {
		t.Errorf("CriticalPathDeltaMs = %f, want 300", diff.CriticalPathDeltaMs)
	}

	if !diff.HasChanges() {
		t.Error("diff with critical-path change should report changes")
	}

	// Symmetry: reversing operands negates the delta.
	if reverse := other.Diff(base); reverse.CriticalPathDeltaMs != -300 {
		t.Errorf("reversed CriticalPathDeltaMs = %f, want -300", reverse.CriticalPathDeltaMs)
	}
}

func TestDiff_CriticalPathStepsAddedRemoved(t *testing.T) {
	t.Parallel()

	// Diff() computes critical-path membership diffs from the precomputed
	// CriticalPathSteps field. Synthetic reports make the set difference
	// deterministic — no dependency on which branch the scheduler picks first.
	r1 := auditlog.WorkflowReport{CriticalPathSteps: []string{"a", "b", "c", "d"}}
	r2 := auditlog.WorkflowReport{CriticalPathSteps: []string{"a", "b", "e"}}

	diff := r1.Diff(r2)

	// r1's path has c and d; r2's does not → removed.
	if len(diff.CriticalPathStepsRemoved) != 2 {
		t.Errorf("CriticalPathStepsRemoved = %v, want [c d]", diff.CriticalPathStepsRemoved)
	}

	// r2's path has e; r1's does not → added.
	if len(diff.CriticalPathStepsAdded) != 1 || diff.CriticalPathStepsAdded[0] != "e" {
		t.Errorf("CriticalPathStepsAdded = %v, want [e]", diff.CriticalPathStepsAdded)
	}

	// Shared steps (a, b) appear in neither slice.
	for _, n := range diff.CriticalPathStepsAdded {
		if n == "a" || n == "b" {
			t.Errorf("shared step %q must not appear in Added", n)
		}
	}

	for _, n := range diff.CriticalPathStepsRemoved {
		if n == "a" || n == "b" {
			t.Errorf("shared step %q must not appear in Removed", n)
		}
	}
}

func TestDiff_PeakConcurrencyDelta(t *testing.T) {
	t.Parallel()

	// Diff() reads the precomputed PeakConcurrency field, so synthetic reports
	// are deterministic — no dependency on scheduler timing (which previously
	// forced a t.Skip under -race or loaded CI).
	r1 := auditlog.WorkflowReport{PeakConcurrency: 3}
	r2 := auditlog.WorkflowReport{PeakConcurrency: 5}

	diff := r1.Diff(r2)

	if diff.PeakConcurrencyDelta != 2 {
		t.Errorf("PeakConcurrencyDelta = %d, want 2", diff.PeakConcurrencyDelta)
	}

	if !diff.HasChanges() {
		t.Error("diff with peak-concurrency change should report changes")
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

func BenchmarkDiff_100Steps(b *testing.B) {
	dur := func(ms float64) *float64 { return &ms }

	makeReport := func(baseDuration float64) auditlog.WorkflowReport {
		steps := make([]auditlog.StepInfo, 0, 100)
		for i := range 100 {
			steps = append(steps, auditlog.StepInfo{
				StepRef:      auditlog.StepRef{Name: fmt.Sprintf("step-%03d", i)},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   dur(baseDuration + float64(i)),
				AttemptCount: 1,
			})
		}

		return auditlog.WorkflowReport{Steps: steps}
	}

	base := makeReport(10)
	other := makeReport(15)

	b.ResetTimer()

	for range b.N {
		_ = base.Diff(other)
	}
}
