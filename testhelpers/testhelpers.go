// Package testhelpers provides shared fixtures and assertions used by the
// auditlog unit tests. It lives in the core module so that both the core
// package tests and the viz sub-module tests can import it without creating a
// circular dependency.
package testhelpers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/cenkalti/backoff/v4"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// --- Test step types (implement String() for clean flow.String output) ---

// SucceedStep is a test step that always succeeds.
type SucceedStep struct {
	Name string
	Ran  bool
}

// Do implements flow.Steper.
func (s *SucceedStep) Do(_ context.Context) error {
	s.Ran = true

	return nil
}

// String returns the step name.
func (s *SucceedStep) String() string { return s.Name }

// FailStep is a test step that always fails with the given message.
type FailStep struct {
	Name string
	Msg  string
}

// Do implements flow.Steper.
func (s *FailStep) Do(_ context.Context) error { return TestError(s.Msg) }

// String returns the step name.
func (s *FailStep) String() string { return s.Name }

// FlakyStep is a test step that fails until it reaches its success threshold.
type FlakyStep struct {
	Name      string
	FailUntil int
	Calls     int
}

// Do implements flow.Steper.
func (s *FlakyStep) Do(_ context.Context) error {
	s.Calls++
	if s.Calls <= s.FailUntil {
		return TestError("transient failure")
	}

	return nil
}

// String returns the step name.
func (s *FlakyStep) String() string { return s.Name }

// SlowStep is a test step that sleeps for the configured duration.
type SlowStep struct {
	Name string
	D    time.Duration
}

// Do implements flow.Steper.
func (s *SlowStep) Do(ctx context.Context) error {
	select {
	case <-time.After(s.D):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("slow step cancelled: %w", ctx.Err())
	}
}

// String returns the step name.
func (s *SlowStep) String() string { return s.Name }

// TestError is a simple error type used by test steps.
type TestError string

// Error returns the error message.
func (e TestError) Error() string { return string(e) }

// --- Step constructors ---

// NewSucceed returns a new SucceedStep with the given name.
func NewSucceed(name string) *SucceedStep { return &SucceedStep{Name: name} }

// NewFail returns a new FailStep with the given name and message.
func NewFail(name, msg string) *FailStep { return &FailStep{Name: name, Msg: msg} }

// NewFlaky returns a new FlakyStep with the given name and failure threshold.
func NewFlaky(name string, failUntil int) *FlakyStep {
	return &FlakyStep{Name: name, FailUntil: failUntil}
}

// NewSlow returns a new SlowStep with the given name and duration.
func NewSlow(name string, d time.Duration) *SlowStep { return &SlowStep{Name: name, D: d} }

// StepFixture builds a StepInfo with just a name and status.
func StepFixture(name string, status auditlog.StepStatus) auditlog.StepInfo {
	return auditlog.StepInfo{StepRef: auditlog.StepRef{Name: name}, Status: status}
}

// RetryOpts returns a retry config function with a FRESH backoff instance,
// avoiding the data race in go-workflow's shared DefaultRetryOption.Backoff.
func RetryOpts(attempts uint64) func(*flow.RetryOption) {
	return func(o *flow.RetryOption) {
		o.Attempts = attempts
		o.Backoff = backoff.NewExponentialBackOff()
	}
}

// AddRetryStep wires a step into the workflow with the given retry attempt count.
func AddRetryStep(w *flow.Workflow, step flow.Steper, attempts uint64) {
	w.Add(flow.Step(step).Retry(RetryOpts(attempts)))
}

// AddDependentStep wires parent and child steps into the workflow, where
// child depends on parent.
func AddDependentStep(w *flow.Workflow, parent, child flow.Steper) {
	w.Add(
		flow.Step(parent),
		flow.Step(child).DependsOn(parent),
	)
}

// AddParallelSteps wires two independent steps into the workflow as a
// parallel pair.
func AddParallelSteps(w *flow.Workflow, a, b flow.Steper) {
	w.Add(flow.Step(a), flow.Step(b))
}

// AddSlowParallelSteps wires two slow steps of the same duration into the
// workflow as a parallel pair.
func AddSlowParallelSteps(w *flow.Workflow, d time.Duration) {
	AddParallelSteps(w, NewSlow("a", d), NewSlow("b", d))
}

// AddLinearChain wires a 3-step linear dependency chain (a → b → c) into the
// workflow.
func AddLinearChain(w *flow.Workflow, a, b, c flow.Steper) {
	w.Add(
		flow.Step(a),
		flow.Step(b).DependsOn(a),
		flow.Step(c).DependsOn(b),
	)
}

// AddSingleStep wires a single succeed step with the given name into the
// workflow.
func AddSingleStep(w *flow.Workflow, name string) {
	w.Add(flow.Step(NewSucceed(name)))
}

// RunSingleSucceed runs a minimal single-succeed-step workflow end-to-end:
// creates the audit+workflow fixture, wires a single step with the given
// name, attaches the auditor, runs the workflow, and snapshots state.
// Returns the auditor for the test to operate on.
func RunSingleSucceed(t *testing.T, name string) *auditlog.Auditor {
	t.Helper()

	a, w := NewAuditAndWorkflow(t)
	AddSingleStep(w, name)
	RunWorkflow(t, a, w)

	return a
}

// RunSingleSucceedWithBuffer is the Write* (non-String) variant of
// RunSingleSucceed: returns the auditor plus a fresh strings.Builder for
// capturing rendered output.
func RunSingleSucceedWithBuffer(t *testing.T, name string) (*auditlog.Auditor, *strings.Builder) {
	t.Helper()

	a := RunSingleSucceed(t, name)

	buf := &strings.Builder{}

	return a, buf
}

// SingleSucceedExportPath is the shared fixture for every Export* test:
// runs a single-succeed workflow with the given step name and returns the
// auditor plus a t.TempDir-anchored output file path.
func SingleSucceedExportPath(t *testing.T, stepName, fileName string) (*auditlog.Auditor, string) {
	t.Helper()

	a := RunSingleSucceed(t, stepName)

	return a, t.TempDir() + "/" + fileName
}

// --- Helpers ---

// MustNew creates an audit log Auditor and fails the test on error.
func MustNew(t *testing.T, cfg auditlog.Config) *auditlog.Auditor {
	t.Helper()

	a, err := auditlog.New(cfg)
	if err != nil {
		t.Fatalf("auditlog.New(%+v) error: %v", cfg, err)
	}

	return a
}

// NewAuditAndWorkflow returns a fresh enabled auditor and empty workflow.
func NewAuditAndWorkflow(t *testing.T) (*auditlog.Auditor, *flow.Workflow) {
	t.Helper()

	a := MustNew(t, auditlog.Config{Enabled: true, WorkflowID: "test"})
	w := &flow.Workflow{} //nolint:exhaustruct

	return a, w
}

// MustNewWithID returns a fresh enabled auditor with the given WorkflowID.
func MustNewWithID(t *testing.T, workflowID string) *auditlog.Auditor {
	t.Helper()

	return MustNew(t, auditlog.Config{Enabled: true, WorkflowID: workflowID})
}

// FindStep returns the StepInfo with the exact name, failing if not found.
func FindStep(t *testing.T, report auditlog.WorkflowReport, name string) auditlog.StepInfo {
	t.Helper()

	for _, s := range report.Steps {
		if s.Name == name {
			return s
		}
	}

	t.Fatalf("step %q not found in report (have %d steps)", name, len(report.Steps))

	return auditlog.StepInfo{}
}

// AssertReportValid fails the test if the report does not validate.
func AssertReportValid(t *testing.T, report auditlog.WorkflowReport) {
	t.Helper()

	err := report.Validate()
	if err != nil {
		t.Fatalf("report invalid: %v", err)
	}
}

// AssertStepCount fails the test if the step count does not match want.
func AssertStepCount(t *testing.T, report auditlog.WorkflowReport, want int) {
	t.Helper()

	if report.StepCount != want {
		t.Fatalf("expected %d steps, got %d", want, report.StepCount)
	}
}

// AssertEventCount fails the test if the event count does not match want.
func AssertEventCount(t *testing.T, report auditlog.WorkflowReport, want int) {
	t.Helper()

	if report.EventCount != want {
		t.Errorf("expected %d events, got %d", want, report.EventCount)
	}
}

// AssertCount fails the test if got does not equal want.
func AssertCount(t *testing.T, name string, got, want int) {
	t.Helper()

	if got != want {
		t.Errorf("expected %s=%d, got %d", name, want, got)
	}
}

// AssertWorkflowID fails the test if the WorkflowID does not match want.
func AssertWorkflowID(t *testing.T, report auditlog.WorkflowReport, want string) {
	t.Helper()

	if report.WorkflowID != want {
		t.Errorf("expected WorkflowID=%q, got %q", want, report.WorkflowID)
	}
}

// AssertAttemptCount fails the test if the step's attempt count does not match want.
func AssertAttemptCount(t *testing.T, s auditlog.StepInfo, want int) {
	t.Helper()

	if s.AttemptCount != want {
		t.Errorf("expected %d attempts, got %d", want, s.AttemptCount)
	}
}

// AssertStatus fails the test if the step's status does not match want.
func AssertStatus(t *testing.T, s auditlog.StepInfo, want auditlog.StepStatus) {
	t.Helper()

	if s.Status != want {
		t.Errorf("expected status %s, got %s", want, s.Status)
	}
}

// AssertContains fails the test if substr is not present in output.
func AssertContains(t *testing.T, output, substr, message string) {
	t.Helper()

	if !strings.Contains(output, substr) {
		t.Error(message)
	}
}

// AssertFirstStepName fails the test if the report's first step is not named want.
func AssertFirstStepName(t *testing.T, report auditlog.WorkflowReport, want string) {
	t.Helper()

	if report.Steps[0].Name != want {
		t.Errorf("expected first step %q, got %q", want, report.Steps[0].Name)
	}
}

// AssertPeakConcurrency fails the test if the report's PeakConcurrency does
// not match want.
func AssertPeakConcurrency(t *testing.T, report auditlog.WorkflowReport, want int) {
	t.Helper()

	if report.PeakConcurrency != want {
		t.Errorf("expected PeakConcurrency=%d, got %d", want, report.PeakConcurrency)
	}
}

// AssertFailureReason fails the test if the report's FailureReason does not
// match want.
func AssertFailureReason(t *testing.T, report auditlog.WorkflowReport, want string) {
	t.Helper()

	if report.FailureReason != want {
		t.Errorf("expected FailureReason=%q, got %q", want, report.FailureReason)
	}
}

// AssertEventRunIDsMatch fails the test if any event does not have the expected RunID.
func AssertEventRunIDsMatch(t *testing.T, events []auditlog.Event, runID auditlog.RunID) {
	t.Helper()

	for i, evt := range events {
		if evt.RunID != runID {
			t.Errorf("event %d RunID %q != %q", i, evt.RunID, runID)
		}
	}
}

// AssertFailedCount fails the test if report.FailedCount does not equal want.
func AssertFailedCount(t *testing.T, report auditlog.WorkflowReport, want int) {
	t.Helper()

	if report.FailedCount != want {
		t.Errorf("expected %d failed, got %d", want, report.FailedCount)
	}
}

// AssertNilStep fails the test if the returned *StepInfo is non-nil.
func AssertNilStep(t *testing.T, got *auditlog.StepInfo, message string) {
	t.Helper()

	if got != nil {
		t.Error(message)
	}
}

// RunWorkflow attaches the auditor, runs the workflow, and snapshots state.
func RunWorkflow(t *testing.T, a *auditlog.Auditor, w *flow.Workflow) {
	t.Helper()

	a.Attach(w)
	_ = w.Do(context.Background())
	a.Snapshot(w)
}

// WriteSingleStepHTML runs a workflow containing exactly the given step and
// renders the resulting audit log to an HTML buffer, returning the rendered
// string.
func WriteSingleStepHTML(t *testing.T, step flow.Steper) string {
	t.Helper()

	a, w := NewAuditAndWorkflow(t)
	w.Add(flow.Step(step))
	RunWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	return buf.String()
}
