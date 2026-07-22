package auditlog_test

import (
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

func TestFiltered_ByStepName(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s1 := testhelpers.NewSucceed("keep-me")
	s2 := testhelpers.NewSucceed("filter-out")
	testhelpers.AddParallelSteps(w, s1, s2)
	testhelpers.RunWorkflow(t, a, w)

	filtered := a.ReportFiltered(auditlog.WithStepsByName("keep-me"))
	testhelpers.AssertStepCount(t, filtered, 1)
	testhelpers.AssertFirstStepName(t, filtered, "keep-me")
}

func TestFiltered_ByStatus(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	ok := testhelpers.NewSucceed("ok")
	bad := testhelpers.NewFail("bad", "err")
	testhelpers.AddParallelSteps(w, ok, bad)
	testhelpers.RunWorkflow(t, a, w)

	filtered := a.ReportFiltered(auditlog.WithStepsByStatus(auditlog.StepStatusFailed))
	testhelpers.AssertStepCount(t, filtered, 1)
	testhelpers.AssertFirstStepName(t, filtered, "bad")
}

func TestFiltered_ByEventType(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewSucceed("filter-event")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

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

	a := testhelpers.RunSingleSucceed(t, "time-step")

	before := time.Now().Add(-1 * time.Hour)
	after := time.Now().Add(1 * time.Hour)

	filtered := a.ReportFiltered(auditlog.WithTimeRange(before, after))
	if filtered.EventCount == 0 {
		t.Error("expected events within time range")
	}
}

func TestFiltered_NoOptions(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s1 := testhelpers.NewSucceed("s1")
	s2 := testhelpers.NewSucceed("s2")
	testhelpers.AddParallelSteps(w, s1, s2)
	testhelpers.RunWorkflow(t, a, w)

	original := a.Report()
	filtered := original.Filtered()

	if filtered.StepCount != original.StepCount {
		t.Errorf("expected %d steps, got %d", original.StepCount, filtered.StepCount)
	}
}

func TestFiltered_MultipleStatuses(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	ok := testhelpers.NewSucceed("ok")
	bad := testhelpers.NewFail("bad", "err")
	testhelpers.AddParallelSteps(w, ok, bad)
	testhelpers.RunWorkflow(t, a, w)

	filtered := a.ReportFiltered(
		auditlog.WithStepsByStatus(auditlog.StepStatusSucceeded, auditlog.StepStatusFailed),
	)
	testhelpers.AssertStepCount(t, filtered, 2)
}

func TestFiltered_EventsFilteredToSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s1 := testhelpers.NewSucceed("keep")
	s2 := testhelpers.NewSucceed("drop")
	testhelpers.AddParallelSteps(w, s1, s2)
	testhelpers.RunWorkflow(t, a, w)

	filtered := a.ReportFiltered(auditlog.WithStepsByName("keep"))

	for _, evt := range filtered.Events {
		if evt.Name == "drop" {
			t.Error("expected no events for 'drop' step")
		}
	}
}

func TestFiltered_RetriesFiltered(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewFlaky("flaky", 2)
	testhelpers.AddRetryStep(w, step, 5)
	testhelpers.RunWorkflow(t, a, w)

	// Filter to only succeeded steps — the flaky step should be included
	// because it eventually succeeded.
	filtered := a.ReportFiltered(auditlog.WithStepsByStatus(auditlog.StepStatusSucceeded))
	testhelpers.AssertStepCount(t, filtered, 1)
}

func TestFiltered_AggregatesRecomputed(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	ok := testhelpers.NewSucceed("ok")
	bad := testhelpers.NewFail("bad", "err")
	testhelpers.AddParallelSteps(w, ok, bad)
	testhelpers.RunWorkflow(t, a, w)

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
