package auditlog_test

import (
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestDiff_NoChanges(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	w.Add(flow.Step(newSucceed("step")))
	runWorkflow(t, a, w)

	r1 := a.Report()
	r2 := a.Report()

	diff := r1.Diff(r2)
	if diff.HasChanges() {
		t.Errorf("expected no changes, got %+v", diff)
	}
}

func TestDiff_StatusChanged(t *testing.T) {
	t.Parallel()

	a1, w1 := newAuditAndWorkflow(t)
	w1.Add(flow.Step(newSucceed("step")))
	runWorkflow(t, a1, w1)

	a2, w2 := newAuditAndWorkflow(t)
	w2.Add(flow.Step(newFail("step", "err")))
	runWorkflow(t, a2, w2)

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
		a, w := newAuditAndWorkflow(t)
		w.Add(
			flow.Step(newSucceed("alpha")),
			flow.Step(newSucceed("bravo")),
			flow.Step(newSucceed("charlie")),
		)
		runWorkflow(t, a, w)

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

	a1, w1 := newAuditAndWorkflow(t)
	w1.Add(flow.Step(newSucceed("a")))
	runWorkflow(t, a1, w1)

	a2, w2 := newAuditAndWorkflow(t)
	w2.Add(flow.Step(newSucceed("a")), flow.Step(newSucceed("b")))
	runWorkflow(t, a2, w2)

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

	a1, w1 := newAuditAndWorkflow(t)
	w1.Add(flow.Step(newSucceed("a")), flow.Step(newSucceed("b")))
	runWorkflow(t, a1, w1)

	a2, w2 := newAuditAndWorkflow(t)
	w2.Add(flow.Step(newSucceed("a")))
	runWorkflow(t, a2, w2)

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

	a1, w1 := newAuditAndWorkflow(t)
	w1.Add(flow.Step(newSucceed("step")))
	runWorkflow(t, a1, w1)

	a2, w2 := newAuditAndWorkflow(t)
	w2.Add(flow.Step(newSlow("step", 20*time.Millisecond)))
	runWorkflow(t, a2, w2)

	diff := a1.Report().Diff(a2.Report())
	if diff.DurationDelta <= 0 {
		t.Errorf("expected positive duration delta, got %f", diff.DurationDelta)
	}
}

func TestDuration_WallClock(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	w.Add(flow.Step(newSlow("slow", 20*time.Millisecond)))
	runWorkflow(t, a, w)

	dur := a.Report().Duration()
	if dur < 15*time.Millisecond {
		t.Errorf("expected duration >= 15ms, got %v", dur)
	}
}

func TestDuration_EmptyReport(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})
	if a.Report().Duration() != 0 {
		t.Error("expected 0 duration for empty report")
	}
}

func TestSummary(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	w.Add(flow.Step(newSucceed("ok")), flow.Step(newFail("bad", "err")))
	runWorkflow(t, a, w)

	summary := a.Report().Summary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}
