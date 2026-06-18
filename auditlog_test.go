package auditlog_test

import (
	"context"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/cenkalti/backoff/v4"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// --- Test step types (implement String() for clean flow.String output) ---

type succeedStep struct {
	name string
	ran  bool
}

func (s *succeedStep) Do(_ context.Context) error { s.ran = true; return nil }
func (s *succeedStep) String() string             { return s.name }

type failStep struct {
	name string
	msg  string
}

func (s *failStep) Do(_ context.Context) error { return errTest(s.msg) }
func (s *failStep) String() string             { return s.name }

type flakyStep struct {
	name      string
	failUntil int
	calls     int
}

func (s *flakyStep) Do(_ context.Context) error {
	s.calls++
	if s.calls <= s.failUntil {
		return errTest("transient failure")
	}

	return nil
}

func (s *flakyStep) String() string { return s.name }

type slowStep struct {
	name string
	d    time.Duration
}

func (s *slowStep) Do(ctx context.Context) error {
	select {
	case <-time.After(s.d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *slowStep) String() string { return s.name }

type errTest string

func (e errTest) Error() string { return string(e) }

// --- Step constructors ---

func newSucceed(name string) *succeedStep { return &succeedStep{name: name} }
func newFail(name, msg string) *failStep  { return &failStep{name: name, msg: msg} }
func newFlaky(name string, failUntil int) *flakyStep {
	return &flakyStep{name: name, failUntil: failUntil}
}

func newSlow(name string, d time.Duration) *slowStep { return &slowStep{name: name, d: d} }

// retryOpts returns a retry config function with a FRESH backoff instance,
// avoiding the data race in go-workflow's shared DefaultRetryOption.Backoff.
func retryOpts(attempts uint64) func(*flow.RetryOption) {
	return func(o *flow.RetryOption) {
		o.Attempts = attempts
		o.Backoff = backoff.NewExponentialBackOff()
	}
}

// --- Helpers ---

func mustNew(t *testing.T, cfg auditlog.Config) *auditlog.Auditor {
	t.Helper()

	a, err := auditlog.New(cfg)
	if err != nil {
		t.Fatalf("auditlog.New(%+v) error: %v", cfg, err)
	}

	return a
}

func newAuditAndWorkflow(t *testing.T) (*auditlog.Auditor, *flow.Workflow) {
	t.Helper()

	a := mustNew(t, auditlog.Config{Enabled: true, WorkflowID: "test"})
	w := &flow.Workflow{}

	return a, w
}

func findStep(t *testing.T, report auditlog.WorkflowReport, name string) auditlog.StepInfo {
	t.Helper()

	for _, s := range report.Steps {
		if s.Name == name {
			return s
		}
	}

	t.Fatalf("step %q not found in report (have %d steps)", name, len(report.Steps))

	return auditlog.StepInfo{}
}

func assertReportValid(t *testing.T, report auditlog.WorkflowReport) {
	t.Helper()

	if err := report.Validate(); err != nil {
		t.Fatalf("report invalid: %v", err)
	}
}

func runWorkflow(t *testing.T, a *auditlog.Auditor, w *flow.Workflow) {
	t.Helper()

	a.Attach(w)
	_ = w.Do(context.Background())
	a.Snapshot(w)
}

// --- Tests ---

func TestNew_DefaultWorkflowID(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})
	report := a.Report()

	if report.WorkflowID != "default" {
		t.Errorf("expected default WorkflowID, got %q", report.WorkflowID)
	}
}

func TestNew_ValidateWorkflowID(t *testing.T) {
	t.Parallel()

	_, err := auditlog.New(auditlog.Config{Enabled: true, WorkflowID: "bad/id"})
	if err == nil {
		t.Fatal("expected error for WorkflowID with path separator")
	}
}

func TestDisabled_IsNoOp(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: false})
	w := &flow.Workflow{}
	step := newSucceed("noop-step")
	w.Add(flow.Step(step))

	a.Attach(w)
	_ = w.Do(context.Background())
	a.Snapshot(w)

	report := a.Report()
	if report.StepCount != 0 {
		t.Errorf("disabled auditor should have 0 steps, got %d", report.StepCount)
	}

	if report.EventCount != 0 {
		t.Errorf("disabled auditor should have 0 events, got %d", report.EventCount)
	}
}

func TestSingleStep_Success(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("my-step")
	w.Add(flow.Step(step))

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	if report.StepCount != 1 {
		t.Fatalf("expected 1 step, got %d", report.StepCount)
	}

	s := findStep(t, report, "my-step")
	if s.Status != auditlog.StepStatusSucceeded {
		t.Errorf("expected succeeded, got %s", s.Status)
	}

	if s.AttemptCount != 1 {
		t.Errorf("expected 1 attempt, got %d", s.AttemptCount)
	}

	if report.SucceededCount != 1 {
		t.Errorf("expected SucceededCount=1, got %d", report.SucceededCount)
	}

	if !report.WorkflowSucceeded {
		t.Error("expected WorkflowSucceeded=true")
	}
}

func TestSingleStep_Failure(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newFail("fail-step", "boom")
	w.Add(flow.Step(step))

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	s := findStep(t, report, "fail-step")
	if s.Status != auditlog.StepStatusFailed {
		t.Errorf("expected failed, got %s", s.Status)
	}

	if s.Error == nil || *s.Error != "boom" {
		t.Errorf("expected error 'boom', got %v", s.Error)
	}

	if report.FailedCount != 1 {
		t.Errorf("expected FailedCount=1, got %d", report.FailedCount)
	}

	if report.WorkflowSucceeded {
		t.Error("expected WorkflowSucceeded=false")
	}
}

func TestDependencies_Tracked(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	save := newSucceed("save")
	w.Add(
		flow.Step(fetch),
		flow.Step(save).DependsOn(fetch),
	)

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	saveStep := findStep(t, report, "save")
	if len(saveStep.Dependencies) != 1 || saveStep.Dependencies[0] != "fetch" {
		t.Errorf("expected save to depend on fetch, got %v", saveStep.Dependencies)
	}

	fetchStep := findStep(t, report, "fetch")
	if len(fetchStep.Dependents) != 1 || fetchStep.Dependents[0] != "save" {
		t.Errorf("expected fetch to be depended on by save, got %v", fetchStep.Dependents)
	}
}

func TestRetry_AttemptCount(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newFlaky("flaky", 2)
	w.Add(
		flow.Step(step).Retry(func(o *flow.RetryOption) {
			o.Attempts = 5
			o.Backoff = backoff.NewExponentialBackOff()
		}),
	)

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	s := findStep(t, report, "flaky")
	if s.Status != auditlog.StepStatusSucceeded {
		t.Errorf("expected succeeded after retries, got %s", s.Status)
	}

	if s.AttemptCount != 3 {
		t.Errorf("expected 3 attempts (2 failures + 1 success), got %d", s.AttemptCount)
	}

	if !s.HasRetry {
		t.Error("expected HasRetry=true")
	}

	if s.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts=5, got %d", s.MaxAttempts)
	}
}

func TestRetry_AllFail(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newFlaky("always-fail", 100)
	w.Add(
		flow.Step(step).Retry(func(o *flow.RetryOption) {
			o.Attempts = 3
			o.Backoff = backoff.NewExponentialBackOff()
		}),
	)

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	s := findStep(t, report, "always-fail")
	if s.Status != auditlog.StepStatusFailed {
		t.Errorf("expected failed, got %s", s.Status)
	}

	if s.AttemptCount != 3 {
		t.Errorf("expected 3 attempts, got %d", s.AttemptCount)
	}
}

func TestSkippedSteps_CapturedBySnapshot(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	upstream := newFail("failing", "fail")
	downstream := newSucceed("skipped")
	w.Add(
		flow.Step(upstream),
		flow.Step(downstream).DependsOn(upstream),
	)

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	skipped := findStep(t, report, "skipped")
	if skipped.Status != auditlog.StepStatusSkipped {
		t.Errorf("expected skipped, got %s", skipped.Status)
	}

	if report.SkippedCount != 1 {
		t.Errorf("expected SkippedCount=1, got %d", report.SkippedCount)
	}
}

func TestEventSequence_Ordered(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("step-a")
	w.Add(flow.Step(step))

	runWorkflow(t, a, w)

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
	a := mustNew(t, auditlog.Config{
		Enabled: true,
		OnEvent: func(e auditlog.Event) { captured = append(captured, e) },
	})
	w := &flow.Workflow{}
	step := newSucceed("step-a")
	w.Add(flow.Step(step))

	runWorkflow(t, a, w)

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

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("export-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	dir := t.TempDir()
	path := dir + "/report.json"
	err := a.ExportToFile(path)
	if err != nil {
		t.Fatalf("ExportToFile error: %v", err)
	}

	report := a.Report()
	if report.Version != auditlog.SchemaVersion {
		t.Errorf("expected version %s, got %s", auditlog.SchemaVersion, report.Version)
	}
}

func TestExport_NDJSON(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("ndjson-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	dir := t.TempDir()
	path := dir + "/events.ndjson"
	err := a.ExportEventsToNDJSON(path)
	if err != nil {
		t.Fatalf("ExportEventsToNDJSON error: %v", err)
	}

	if a.EventsCount() < 2 {
		t.Errorf("expected at least 2 events, got %d", a.EventsCount())
	}
}

func TestEventsByStep(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("query-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	report := a.Report()
	evts := report.EventsByStep("query-step")
	if len(evts) < 2 {
		t.Errorf("expected at least 2 events for query-step, got %d", len(evts))
	}
}

func TestFailedSteps(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ok := newSucceed("ok")
	bad := newFail("bad", "err")
	w.Add(
		flow.Step(ok),
		flow.Step(bad),
	)
	runWorkflow(t, a, w)

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

	a, w := newAuditAndWorkflow(t)
	step := newFlaky("flaky", 1)
	w.Add(
		flow.Step(step).Retry(func(o *flow.RetryOption) {
			o.Attempts = 5
			o.Backoff = backoff.NewExponentialBackOff()
		}),
	)
	runWorkflow(t, a, w)

	report := a.Report()
	retried := report.RetriedSteps()
	if len(retried) != 1 {
		t.Fatalf("expected 1 retried step, got %d", len(retried))
	}

	if retried[0].AttemptCount != 2 {
		t.Errorf("expected 2 attempts, got %d", retried[0].AttemptCount)
	}
}

func TestConcurrentSteps(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSlow("parallel-1", 10*time.Millisecond)
	s2 := newSlow("parallel-2", 10*time.Millisecond)
	s3 := newSlow("parallel-3", 10*time.Millisecond)
	w.Add(
		flow.Step(s1),
		flow.Step(s2),
		flow.Step(s3),
	)
	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	if report.StepCount != 3 {
		t.Fatalf("expected 3 steps, got %d", report.StepCount)
	}

	if report.SucceededCount != 3 {
		t.Errorf("expected 3 succeeded, got %d", report.SucceededCount)
	}
}

func TestTimeout_Configured(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSlow("timeout-step", 5*time.Second)
	w.Add(
		flow.Step(step).Timeout(50 * time.Millisecond),
	)
	runWorkflow(t, a, w)

	report := a.Report()
	s := findStep(t, report, "timeout-step")

	if !s.HasTimeout {
		t.Error("expected HasTimeout=true")
	}

	if s.Status != auditlog.StepStatusCanceled {
		t.Errorf("expected canceled due to timeout, got %s", s.Status)
	}
}

func TestPipe_DependencyChain(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSucceed("pipe-1")
	s2 := newSucceed("pipe-2")
	s3 := newSucceed("pipe-3")
	w.Add(
		flow.Pipe(s1, s2, s3),
	)
	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	pipe2 := findStep(t, report, "pipe-2")
	if len(pipe2.Dependencies) != 1 || pipe2.Dependencies[0] != "pipe-1" {
		t.Errorf("pipe-2 should depend on pipe-1, got %v", pipe2.Dependencies)
	}

	if len(pipe2.Dependents) != 1 || pipe2.Dependents[0] != "pipe-3" {
		t.Errorf("pipe-2 should be depended on by pipe-3, got %v", pipe2.Dependents)
	}
}

func TestMaxEvents_DropsExcess(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{
		Enabled:   true,
		MaxEvents: 1,
	})
	w := &flow.Workflow{}
	step := newSucceed("capped-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	if a.EventsCount() > 1 {
		t.Errorf("expected at most 1 event, got %d", a.EventsCount())
	}

	if a.DroppedEventCount() < 1 {
		t.Errorf("expected at least 1 dropped event, got %d", a.DroppedEventCount())
	}
}

func TestEnvEnabled(t *testing.T) {
	t.Setenv(auditlog.EnvKeyEnabled, "true")

	a := mustNew(t, auditlog.Config{})
	w := &flow.Workflow{}
	step := newSucceed("env-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	if a.EventsCount() == 0 {
		t.Error("expected events when enabled via env var")
	}
}
