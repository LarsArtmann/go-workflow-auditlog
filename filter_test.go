package auditlog_test

import (
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestFiltered_ByStepName(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSucceed("keep-me")
	s2 := newSucceed("filter-out")
	addParallelSteps(w, s1, s2)
	runWorkflow(t, a, w)

	filtered := a.ReportFiltered(auditlog.WithStepsByName("keep-me"))
	assertStepCount(t, filtered, 1)
	assertFirstStepName(t, filtered, "keep-me")
}

func TestFiltered_ByStatus(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ok := newSucceed("ok")
	bad := newFail("bad", "err")
	addParallelSteps(w, ok, bad)
	runWorkflow(t, a, w)

	filtered := a.ReportFiltered(auditlog.WithStepsByStatus(auditlog.StepStatusFailed))
	assertStepCount(t, filtered, 1)
	assertFirstStepName(t, filtered, "bad")
}

func TestFiltered_ByEventType(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSucceed("filter-event")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	fullReport := a.Report()
	filtered := fullReport.Filtered(auditlog.WithEventsByType(auditlog.EventTypeAttemptStart))

	// Only start events should survive.
	for _, evt := range filtered.Events {
		if evt.EventType != auditlog.EventTypeAttemptStart {
			t.Errorf("expected only attempt_start events, got %s", evt.EventType)
		}
	}

	if len(filtered.Events) == 0 {
		t.Error("expected at least 1 event")
	}
}

func TestFiltered_ByTimeRange(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSucceed("time-step")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	before := time.Now().Add(-1 * time.Hour)
	after := time.Now().Add(1 * time.Hour)

	filtered := a.ReportFiltered(auditlog.WithTimeRange(before, after))
	if filtered.EventCount == 0 {
		t.Error("expected events within time range")
	}
}

func TestFiltered_NoOptions(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSucceed("s1")
	s2 := newSucceed("s2")
	addParallelSteps(w, s1, s2)
	runWorkflow(t, a, w)

	original := a.Report()
	filtered := original.Filtered()

	if filtered.StepCount != original.StepCount {
		t.Errorf("expected %d steps, got %d", original.StepCount, filtered.StepCount)
	}
}

func TestFiltered_MultipleStatuses(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ok := newSucceed("ok")
	bad := newFail("bad", "err")
	addParallelSteps(w, ok, bad)
	runWorkflow(t, a, w)

	filtered := a.ReportFiltered(
		auditlog.WithStepsByStatus(auditlog.StepStatusSucceeded, auditlog.StepStatusFailed),
	)
	assertStepCount(t, filtered, 2)
}

func TestFiltered_EventsFilteredToSteps(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSucceed("keep")
	s2 := newSucceed("drop")
	addParallelSteps(w, s1, s2)
	runWorkflow(t, a, w)

	filtered := a.ReportFiltered(auditlog.WithStepsByName("keep"))

	for _, evt := range filtered.Events {
		if evt.Name == "drop" {
			t.Error("expected no events for 'drop' step")
		}
	}
}

func TestFiltered_RetriesFiltered(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newFlaky("flaky", 2)
	addRetryStep(w, step, 5)
	runWorkflow(t, a, w)

	// Filter to only succeeded steps — the flaky step should be included
	// because it eventually succeeded.
	filtered := a.ReportFiltered(auditlog.WithStepsByStatus(auditlog.StepStatusSucceeded))
	assertStepCount(t, filtered, 1)
}

func TestFiltered_AggregatesRecomputed(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ok := newSucceed("ok")
	bad := newFail("bad", "err")
	addParallelSteps(w, ok, bad)
	runWorkflow(t, a, w)

	full := a.Report()
	filtered := full.Filtered(auditlog.WithStepsByName("ok"))

	// The filtered report should have recomputed SucceededCount.
	if filtered.SucceededCount != 1 {
		t.Errorf("expected SucceededCount=1, got %d", filtered.SucceededCount)
	}

	if filtered.FailedCount != 0 {
		t.Errorf("expected FailedCount=0, got %d", filtered.FailedCount)
	}
}
