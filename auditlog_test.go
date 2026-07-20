package auditlog_test

import (
	"context"
	"errors"
	"strings"
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

func (s *failStep) Do(_ context.Context) error { return testError(s.msg) }
func (s *failStep) String() string             { return s.name }

type flakyStep struct {
	name      string
	failUntil int
	calls     int
}

func (s *flakyStep) Do(_ context.Context) error {
	s.calls++
	if s.calls <= s.failUntil {
		return testError("transient failure")
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

type testError string

func (e testError) Error() string { return string(e) }

// --- Step constructors ---

func newSucceed(name string) *succeedStep { return &succeedStep{name: name} }
func newFail(name, msg string) *failStep  { return &failStep{name: name, msg: msg} }
func newFlaky(name string, failUntil int) *flakyStep {
	return &flakyStep{name: name, failUntil: failUntil}
}

func newSlow(name string, d time.Duration) *slowStep { return &slowStep{name: name, d: d} }

// stepFixture builds a StepInfo with just a name and status. Centralizes the
// {StepRef: auditlog.StepRef{Name: ...}, Status: ...} struct literal that
// dozens of tests use to construct minimal step records.
func stepFixture(name string, status auditlog.StepStatus) auditlog.StepInfo {
	return auditlog.StepInfo{StepRef: auditlog.StepRef{Name: name}, Status: status}
}

// retryOpts returns a retry config function with a FRESH backoff instance,
// avoiding the data race in go-workflow's shared DefaultRetryOption.Backoff.
func retryOpts(attempts uint64) func(*flow.RetryOption) {
	return func(o *flow.RetryOption) {
		o.Attempts = attempts
		o.Backoff = backoff.NewExponentialBackOff()
	}
}

// addRetryStep wires a step into the workflow with the given retry attempt count.
// Centralizes the boilerplate of flow.Step(x).Retry(retryOpts(N)) so individual
// tests don't have to repeat it.
func addRetryStep(w *flow.Workflow, step flow.Steper, attempts uint64) {
	w.Add(flow.Step(step).Retry(retryOpts(attempts)))
}

// addDependentStep wires parent and child steps into the workflow, where
// child depends on parent. Centralizes the two-step dependency chain so tests
// don't repeat the boilerplate.
func addDependentStep(w *flow.Workflow, parent, child flow.Steper) {
	w.Add(
		flow.Step(parent),
		flow.Step(child).DependsOn(parent),
	)
}

// addParallelSteps wires two independent steps into the workflow as a
// parallel pair. Centralizes the w.Add(flow.Step(x), flow.Step(y)) idiom
// used by tests that need a 2-step setup with no dependency edge.
func addParallelSteps(w *flow.Workflow, a, b flow.Steper) {
	w.Add(flow.Step(a), flow.Step(b))
}

// addSlowParallelSteps wires two slow steps of the same duration into the
// workflow as a parallel pair. Centralizes the
// `addParallelSteps(w, newSlow("a", D), newSlow("b", D))` idiom used by
// wall-clock and concurrency tests that need guaranteed overlapping execution.
func addSlowParallelSteps(w *flow.Workflow, d time.Duration) {
	addParallelSteps(w, newSlow("a", d), newSlow("b", d))
}

// addLinearChain wires a 3-step linear dependency chain (a → b → c) into the
// workflow. Centralizes the `w.Add(flow.Step(a), flow.Step(b).DependsOn(a),
// flow.Step(c).DependsOn(b))` idiom used by diagram and HTML tests that need
// a sequential pipeline with clear directional flow.
func addLinearChain(w *flow.Workflow, a, b, c flow.Steper) {
	w.Add(
		flow.Step(a),
		flow.Step(b).DependsOn(a),
		flow.Step(c).DependsOn(b),
	)
}

// addSingleStep wires a single succeed step with the given name into the
// workflow. Centralizes the `step := newSucceed(name); w.Add(flow.Step(step))`
// idiom used by tests that only need a minimal single-step workflow (the
// overwhelming majority of export/format tests).
func addSingleStep(w *flow.Workflow, name string) {
	w.Add(flow.Step(newSucceed(name)))
}

// runSingleSucceed runs a minimal single-succeed-step workflow end-to-end:
// creates the audit+workflow fixture, wires a single step with the given
// name, attaches the auditor, runs the workflow, and snapshots state.
// Returns the auditor for the test to operate on. Centralizes the 4-line
// `t.Parallel + newAuditAndWorkflow + addSingleStep + runWorkflow` boilerplate
// shared by every export/format test.
func runSingleSucceed(t *testing.T, name string) *auditlog.Auditor {
	t.Helper()

	a, w := newAuditAndWorkflow(t)
	addSingleStep(w, name)
	runWorkflow(t, a, w)

	return a
}

// singleSucceedExportPath is the shared fixture for every Export* test:
// runs a single-succeed workflow with the given step name and returns the
// auditor plus a t.TempDir-anchored output file path. Centralizes the
// 2-line `runSingleSucceed + t.TempDir + path` block duplicated across every
// format export test (Mermaid, PlantUML, Graphviz, D2, JSON, HTML, table,
// tree, HTML tree).
//
// Callers are responsible for invoking t.Parallel() before calling this
// helper — keeping the parallel call at the test level satisfies the
// paralleltest linter and keeps the test's parallel participation visible.
func singleSucceedExportPath(t *testing.T, stepName, fileName string) (*auditlog.Auditor, string) {
	t.Helper()

	a := runSingleSucceed(t, stepName)

	return a, t.TempDir() + "/" + fileName
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
		t.Fatalf("expected %d steps, got %d", want, report.StepCount)
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

// assertContains fails the test if substr is not present in output.
func assertContains(t *testing.T, output, substr, message string) {
	t.Helper()

	if !strings.Contains(output, substr) {
		t.Error(message)
	}
}

// assertFirstStepName fails the test if the report's first step is not named want.
func assertFirstStepName(t *testing.T, report auditlog.WorkflowReport, want string) {
	t.Helper()

	if report.Steps[0].Name != want {
		t.Errorf("expected first step %q, got %q", want, report.Steps[0].Name)
	}
}

// assertPeakConcurrency fails the test if the report's PeakConcurrency does
// not match want. Used by peak-concurrency coverage tests.
func assertPeakConcurrency(t *testing.T, report auditlog.WorkflowReport, want int) {
	t.Helper()

	if report.PeakConcurrency != want {
		t.Errorf("expected PeakConcurrency=%d, got %d", want, report.PeakConcurrency)
	}
}

// assertFailureReason fails the test if the report's FailureReason does not
// match want. Used by failure-reason coverage tests.
func assertFailureReason(t *testing.T, report auditlog.WorkflowReport, want string) {
	t.Helper()

	if report.FailureReason != want {
		t.Errorf("expected FailureReason=%q, got %q", want, report.FailureReason)
	}
}

// assertEventRunIDsMatch fails the test if any event in events does not have
// the expected RunID. Used by RunID propagation tests.
func assertEventRunIDsMatch(t *testing.T, events []auditlog.Event, runID auditlog.RunID) {
	t.Helper()

	for i, evt := range events {
		if evt.RunID != runID {
			t.Errorf("event %d RunID %q != %q", i, evt.RunID, runID)
		}
	}
}

// assertFailedCount fails the test if report.FailedCount does not equal want.
// Used by failure count assertions across acceptance and replay tests.
func assertFailedCount(t *testing.T, report auditlog.WorkflowReport, want int) {
	t.Helper()

	if report.FailedCount != want {
		t.Errorf("expected %d failed, got %d", want, report.FailedCount)
	}
}

// assertNilStep fails the test if the returned *StepInfo is non-nil, with a
// message explaining what was looked up. Centralizes the
// "index returned non-nil for unknown X" idiom used by ReportIndex tests.
func assertNilStep(t *testing.T, got *auditlog.StepInfo, message string) {
	t.Helper()

	if got != nil {
		t.Error(message)
	}
}

func runWorkflow(t *testing.T, a *auditlog.Auditor, w *flow.Workflow) {
	t.Helper()

	a.Attach(w)
	_ = w.Do(context.Background())
	a.Snapshot(w)
}

// writeSingleStepHTML runs a workflow containing exactly the given step and
// renders the resulting audit log to an HTML buffer, returning the rendered
// string. Centralizes the
// `newAuditAndWorkflow + w.Add(flow.Step(step)) + runWorkflow + WriteHTML`
// boilerplate shared by 8 dashboard tests that each assert on rendered HTML
// for a single-step workflow.
func writeSingleStepHTML(t *testing.T, step flow.Steper) string {
	t.Helper()

	a, w := newAuditAndWorkflow(t)
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	return buf.String()
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

			if !errors.Is(err, auditlog.ErrWorkflowIDPathSep) {
				t.Errorf("expected error to wrap auditlog.ErrWorkflowIDPathSep, got: %v", err)
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
	addDependentStep(w, fetch, save)

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
	addRetryStep(w, step, 5)

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

// TestRetry_StepErrorClearedOnSuccess is a regression test: when a step
// fails on earlier attempts and then succeeds, its StepInfo.Error field
// must be nil. The per-attempt error history is preserved in the event
// stream; the step-level Error represents the final outcome only.
func TestRetry_StepErrorClearedOnSuccess(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newFlaky("flaky-clears-error", 2) // fails twice, then succeeds
	addRetryStep(w, step, 5)

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	s := findStep(t, report, "flaky-clears-error")
	assertStatus(t, s, auditlog.StepStatusSucceeded)
	assertAttemptCount(t, s, 3)

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

	a, w := newAuditAndWorkflow(t)
	step := newFlaky("always-fail", 100)
	addRetryStep(w, step, 3)

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

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("ndjson-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

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
	addParallelSteps(w, ok, bad)
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
	addRetryStep(w, step, 5)
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
