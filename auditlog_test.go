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

func (s *succeedStep) Do(_ context.Context) error {
	s.ran = true

	return nil
}
func (s *succeedStep) String() string { return s.name }

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

func mustNewWithID(t *testing.T, workflowID string) *auditlog.Auditor {
	t.Helper()

	return mustNew(t, auditlog.Config{Enabled: true, WorkflowID: workflowID})
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

	err := report.Validate()
	if err != nil {
		t.Fatalf("report invalid: %v", err)
	}
}

func assertStepCount(t *testing.T, report auditlog.WorkflowReport, want int) {
	t.Helper()

	if report.StepCount != want {
		t.Errorf("expected %d steps, got %d", want, report.StepCount)
	}
}

func assertEventCount(t *testing.T, report auditlog.WorkflowReport, want int) {
	t.Helper()

	if report.EventCount != want {
		t.Errorf("expected %d events, got %d", want, report.EventCount)
	}
}

func assertCount(t *testing.T, name string, got, want int) {
	t.Helper()

	if got != want {
		t.Errorf("expected %s=%d, got %d", name, want, got)
	}
}

func assertWorkflowID(t *testing.T, report auditlog.WorkflowReport, want string) {
	t.Helper()

	if report.WorkflowID != want {
		t.Errorf("expected WorkflowID=%q, got %q", want, report.WorkflowID)
	}
}

func assertAttemptCount(t *testing.T, s auditlog.StepInfo, want int) {
	t.Helper()

	if s.AttemptCount != want {
		t.Errorf("expected %d attempts, got %d", want, s.AttemptCount)
	}
}

func assertStatus(t *testing.T, s auditlog.StepInfo, want auditlog.StepStatus) {
	t.Helper()

	if s.Status != want {
		t.Errorf("expected status %s, got %s", want, s.Status)
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

	assertWorkflowID(t, report, "default")
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
		})
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
	assertStepCount(t, report, 0)

	assertEventCount(t, report, 0)
}

func TestSingleStep_Success(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("my-step")
	w.Add(flow.Step(step))

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	assertStepCount(t, report, 1)

	s := findStep(t, report, "my-step")
	assertStatus(t, s, auditlog.StepStatusSucceeded)

	assertAttemptCount(t, s, 1)

	assertCount(t, "SucceededCount", report.SucceededCount, 1)

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
	assertStatus(t, s, auditlog.StepStatusFailed)

	if s.Error == nil || *s.Error != "boom" {
		t.Errorf("expected error 'boom', got %v", s.Error)
	}

	assertCount(t, "FailedCount", report.FailedCount, 1)

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
	if len(saveStep.Dependencies) != 1 || saveStep.Dependencies[0].Name != "fetch" {
		t.Errorf("expected save to depend on fetch, got %v", saveStep.Dependencies)
	}

	fetchStep := findStep(t, report, "fetch")
	if len(fetchStep.Dependents) != 1 || fetchStep.Dependents[0].Name != "save" {
		t.Errorf("expected fetch to be depended on by save, got %v", fetchStep.Dependents)
	}
}

func TestRetry_AttemptCount(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newFlaky("flaky", 2)
	w.Add(
		flow.Step(step).Retry(retryOpts(5)),
	)

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	s := findStep(t, report, "flaky")
	if s.Status != auditlog.StepStatusSucceeded {
		t.Errorf("expected succeeded after retries, got %s", s.Status)
	}

	assertAttemptCount(t, s, 3)

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
		flow.Step(step).Retry(retryOpts(3)),
	)

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	s := findStep(t, report, "always-fail")
	assertStatus(t, s, auditlog.StepStatusFailed)

	assertAttemptCount(t, s, 3)
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

	assertCount(t, "SkippedCount", report.SkippedCount, 1)
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
		flow.Step(step).Retry(retryOpts(5)),
	)
	runWorkflow(t, a, w)

	report := a.Report()

	retried := report.RetriedSteps()
	if len(retried) != 1 {
		t.Fatalf("expected 1 retried step, got %d", len(retried))
	}

	assertAttemptCount(t, retried[0], 2)
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

	assertStepCount(t, report, 3)

	assertCount(t, "SucceededCount", report.SucceededCount, 3)
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
	if len(pipe2.Dependencies) != 1 || pipe2.Dependencies[0].Name != "pipe-1" {
		t.Errorf("pipe-2 should depend on pipe-1, got %v", pipe2.Dependencies)
	}

	if len(pipe2.Dependents) != 1 || pipe2.Dependents[0].Name != "pipe-3" {
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
