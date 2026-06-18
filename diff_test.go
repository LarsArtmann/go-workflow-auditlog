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
