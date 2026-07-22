package auditlog_test

import (
	"context"
	"errors"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

// --- Tests ---

func TestNew_DefaultWorkflowID(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})
	report := a.Report()

	testhelpers.AssertWorkflowID(t, report, "default")
}

func TestNew_ValidateWorkflowID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		workflowID string
	}{
		{"path-separator", "bad/id"},
		{"backslash", "bad\\id"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := auditlog.New(auditlog.Config{Enabled: true, WorkflowID: tc.workflowID})
			if err == nil {
				t.Fatalf("expected error for WorkflowID %q", tc.workflowID)
			}

			if !errors.Is(err, auditlog.ErrWorkflowIDPathSep) {
				t.Errorf("expected error to wrap auditlog.ErrWorkflowIDPathSep, got: %v", err)
			}
		})
	}
}

func TestDisabled_IsNoOp(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: false})
	w := &flow.Workflow{}
	step := testhelpers.NewSucceed("noop-step")
	w.Add(flow.Step(step))

	a.Attach(w)
	_ = w.Do(context.Background())
	a.Snapshot(w)

	report := a.Report()
	testhelpers.AssertStepCount(t, report, 0)

	testhelpers.AssertEventCount(t, report, 0)
}

func TestSingleStep_Success(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewSucceed("my-step")
	w.Add(flow.Step(step))

	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	testhelpers.AssertStepCount(t, report, 1)

	s := testhelpers.FindStep(t, report, "my-step")
	testhelpers.AssertStatus(t, s, auditlog.StepStatusSucceeded)

	testhelpers.AssertAttemptCount(t, s, 1)

	testhelpers.AssertCount(t, "SucceededCount", report.SucceededCount, 1)

	if !report.WorkflowSucceeded {
		t.Error("expected WorkflowSucceeded=true")
	}
}

func TestSingleStep_Failure(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewFail("fail-step", "boom")
	w.Add(flow.Step(step))

	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	s := testhelpers.FindStep(t, report, "fail-step")
	testhelpers.AssertStatus(t, s, auditlog.StepStatusFailed)

	if s.Error == nil || *s.Error != "boom" {
		t.Errorf("expected error 'boom', got %v", s.Error)
	}

	testhelpers.AssertCount(t, "FailedCount", report.FailedCount, 1)

	if report.WorkflowSucceeded {
		t.Error("expected WorkflowSucceeded=false")
	}
}

func TestDependencies_Tracked(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	save := testhelpers.NewSucceed("save")
	testhelpers.AddDependentStep(w, fetch, save)

	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	saveStep := testhelpers.FindStep(t, report, "save")
	if len(saveStep.Dependencies) != 1 || saveStep.Dependencies[0].Name != "fetch" {
		t.Errorf("expected save to depend on fetch, got %v", saveStep.Dependencies)
	}

	fetchStep := testhelpers.FindStep(t, report, "fetch")
	if len(fetchStep.Dependents) != 1 || fetchStep.Dependents[0].Name != "save" {
		t.Errorf("expected fetch to be depended on by save, got %v", fetchStep.Dependents)
	}
}

func TestRetry_AttemptCount(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewFlaky("flaky", 2)
	testhelpers.AddRetryStep(w, step, 5)

	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	s := testhelpers.FindStep(t, report, "flaky")
	if s.Status != auditlog.StepStatusSucceeded {
		t.Errorf("expected succeeded after retries, got %s", s.Status)
	}

	testhelpers.AssertAttemptCount(t, s, 3)

	if !s.HasRetry {
		t.Error("expected HasRetry=true")
	}

	if s.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts=5, got %d", s.MaxAttempts)
	}
}

// TestRetry_StepErrorClearedOnSuccess is a regression test: when a step
// fails on earlier attempts and then succeeds, its StepInfo.Error field
// must be nil. The per-attempt error history is preserved in the event
// stream; the step-level Error represents the final outcome only.
func TestRetry_StepErrorClearedOnSuccess(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewFlaky("flaky-clears-error", 2) // fails twice, then succeeds
	testhelpers.AddRetryStep(w, step, 5)

	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	s := testhelpers.FindStep(t, report, "flaky-clears-error")
	testhelpers.AssertStatus(t, s, auditlog.StepStatusSucceeded)
	testhelpers.AssertAttemptCount(t, s, 3)

	if s.HasError() {
		t.Errorf("expected Error=nil for step that eventually succeeded, got %q", *s.Error)
	}

	// The per-attempt error is still preserved in the event stream.
	endEvents := report.EventsByStep("flaky-clears-error")
	if len(endEvents) == 0 {
		t.Fatal("expected attempt_end events for the flaky step")
	}

	foundTransient := false

	for _, evt := range endEvents {
		if !evt.IsAttemptEnd() {
			continue
		}

		if evt.HasError() {
			foundTransient = true
		}
	}

	if !foundTransient {
		t.Error("expected at least one attempt_end event with the transient error in the event stream")
	}
}

func TestRetry_AllFail(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewFlaky("always-fail", 100)
	testhelpers.AddRetryStep(w, step, 3)

	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	s := testhelpers.FindStep(t, report, "always-fail")
	testhelpers.AssertStatus(t, s, auditlog.StepStatusFailed)

	testhelpers.AssertAttemptCount(t, s, 3)
}

func TestSkippedSteps_CapturedBySnapshot(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	upstream := testhelpers.NewFail("failing", "fail")
	downstream := testhelpers.NewSucceed("skipped")
	w.Add(
		flow.Step(upstream),
		flow.Step(downstream).DependsOn(upstream),
	)

	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	skipped := testhelpers.FindStep(t, report, "skipped")
	if skipped.Status != auditlog.StepStatusSkipped {
		t.Errorf("expected skipped, got %s", skipped.Status)
	}

	testhelpers.AssertCount(t, "SkippedCount", report.SkippedCount, 1)
}

func TestEventSequence_Ordered(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewSucceed("step-a")
	w.Add(flow.Step(step))

	testhelpers.RunWorkflow(t, a, w)

	events := a.Events()
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	for i, evt := range events {
		if evt.Sequence != i+1 {
			t.Errorf("event %d: expected sequence %d, got %d", i, i+1, evt.Sequence)
		}
	}

	if events[0].EventType != auditlog.EventTypeAttemptStart {
		t.Errorf("first event should be attempt_start, got %s", events[0].EventType)
	}

	last := events[len(events)-1]
	if last.EventType != auditlog.EventTypeAttemptEnd {
		t.Errorf("last event should be attempt_end, got %s", last.EventType)
	}
}

func TestOnEvent_Callback(t *testing.T) {
	t.Parallel()

	var captured []auditlog.Event

	a := testhelpers.MustNew(t, auditlog.Config{
		Enabled: true,
		OnEvent: func(e auditlog.Event) { captured = append(captured, e) },
	})
	w := &flow.Workflow{}
	step := testhelpers.NewSucceed("step-a")
	w.Add(flow.Step(step))

	testhelpers.RunWorkflow(t, a, w)

	if len(captured) == 0 {
		t.Fatal("OnEvent callback never fired")
	}

	recorderEvents := a.Events()
	if len(captured) != len(recorderEvents) {
		t.Errorf("callback got %d events, recorder has %d", len(captured), len(recorderEvents))
	}
}

func TestExport_JSON(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewSucceed("export-step")
	w.Add(flow.Step(step))
	testhelpers.RunWorkflow(t, a, w)

	dir := t.TempDir()
	path := dir + "/report.json"

	err := a.ExportJSON(path)
	if err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}

	report := a.Report()
	if report.Version != auditlog.SchemaVersion {
		t.Errorf("expected version %s, got %s", auditlog.SchemaVersion, report.Version)
	}
}

func TestExport_NDJSON(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewSucceed("ndjson-step")
	w.Add(flow.Step(step))
	testhelpers.RunWorkflow(t, a, w)

	dir := t.TempDir()
	path := dir + "/events.ndjson"

	err := a.ExportNDJSON(path)
	if err != nil {
		t.Fatalf("ExportNDJSON error: %v", err)
	}

	assertEventsRecorded(t, a, 2)
}

func assertEventsRecorded(t *testing.T, a *auditlog.Auditor, want int) {
	t.Helper()

	if got := a.EventsCount(); got < want {
		t.Errorf("expected at least %d events, got %d", want, got)
	}
}

func TestEventsByStep(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewSucceed("query-step")
	w.Add(flow.Step(step))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	evts := report.EventsByStep("query-step")
	if len(evts) < 2 {
		t.Errorf("expected at least 2 events for query-step, got %d", len(evts))
	}
}

func TestFailedSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	ok := testhelpers.NewSucceed("ok")
	bad := testhelpers.NewFail("bad", "err")
	testhelpers.AddParallelSteps(w, ok, bad)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	failed := report.FailedSteps()
	if len(failed) != 1 {
		t.Fatalf("expected 1 failed step, got %d", len(failed))
	}

	if failed[0].Name != "bad" {
		t.Errorf("expected failed step 'bad', got %q", failed[0].Name)
	}
}

func TestRetriedSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewFlaky("flaky", 1)
	testhelpers.AddRetryStep(w, step, 5)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	retried := report.RetriedSteps()
	if len(retried) != 1 {
		t.Fatalf("expected 1 retried step, got %d", len(retried))
	}

	testhelpers.AssertAttemptCount(t, retried[0], 2)
}

func TestConcurrentSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s1 := testhelpers.NewSlow("parallel-1", 10*time.Millisecond)
	s2 := testhelpers.NewSlow("parallel-2", 10*time.Millisecond)
	s3 := testhelpers.NewSlow("parallel-3", 10*time.Millisecond)
	w.Add(
		flow.Step(s1),
		flow.Step(s2),
		flow.Step(s3),
	)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	testhelpers.AssertStepCount(t, report, 3)

	testhelpers.AssertCount(t, "SucceededCount", report.SucceededCount, 3)
}

func TestTimeout_Configured(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewSlow("timeout-step", 5*time.Second)
	w.Add(
		flow.Step(step).Timeout(50 * time.Millisecond),
	)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	s := testhelpers.FindStep(t, report, "timeout-step")

	if !s.HasTimeout {
		t.Error("expected HasTimeout=true")
	}

	if s.Status != auditlog.StepStatusCanceled {
		t.Errorf("expected canceled due to timeout, got %s", s.Status)
	}
}

func TestPipe_DependencyChain(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s1 := testhelpers.NewSucceed("pipe-1")
	s2 := testhelpers.NewSucceed("pipe-2")
	s3 := testhelpers.NewSucceed("pipe-3")
	w.Add(
		flow.Pipe(s1, s2, s3),
	)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	pipe2 := testhelpers.FindStep(t, report, "pipe-2")
	if len(pipe2.Dependencies) != 1 || pipe2.Dependencies[0].Name != "pipe-1" {
		t.Errorf("pipe-2 should depend on pipe-1, got %v", pipe2.Dependencies)
	}

	if len(pipe2.Dependents) != 1 || pipe2.Dependents[0].Name != "pipe-3" {
		t.Errorf("pipe-2 should be depended on by pipe-3, got %v", pipe2.Dependents)
	}
}

func TestMaxEvents_DropsExcess(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{
		Enabled:   true,
		MaxEvents: 1,
	})
	w := &flow.Workflow{}
	step := testhelpers.NewSucceed("capped-step")
	w.Add(flow.Step(step))
	testhelpers.RunWorkflow(t, a, w)

	if a.EventsCount() > 1 {
		t.Errorf("expected at most 1 event, got %d", a.EventsCount())
	}

	if a.DroppedEventCount() < 1 {
		t.Errorf("expected at least 1 dropped event, got %d", a.DroppedEventCount())
	}
}

func TestEnvEnabled(t *testing.T) {
	t.Setenv(auditlog.EnvKeyEnabled, "true")

	a := testhelpers.MustNew(t, auditlog.Config{})
	w := &flow.Workflow{}
	step := testhelpers.NewSucceed("env-step")
	w.Add(flow.Step(step))
	testhelpers.RunWorkflow(t, a, w)

	if a.EventsCount() == 0 {
		t.Error("expected events when enabled via env var")
	}
}
