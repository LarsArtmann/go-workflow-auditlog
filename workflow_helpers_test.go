package auditlog_test

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

// timeoutStep always returns DeadlineExceeded to simulate a timeout failure.
type timeoutStep struct {
	name string
}

func (s *timeoutStep) Do(_ context.Context) error {
	return context.DeadlineExceeded
}

func (s *timeoutStep) String() string { return s.name }

func TestReport_RetriedStepCount_NoRetries(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)

	a.Attach(w)

	w.Add(flow.Step(testhelpers.NewSucceed("a")))

	testhelpers.RunWorkflow(t, a, w)

	r := a.Report()
	if r.RetriedStepCount() != 0 {
		t.Errorf("RetriedStepCount = %d, want 0", r.RetriedStepCount())
	}

	if r.HasWorkflowRetries() {
		t.Error("HasWorkflowRetries should be false for a single-succeed workflow")
	}

	if r.TotalRetryAttempts() != 0 {
		t.Errorf("TotalRetryAttempts = %d, want 0", r.TotalRetryAttempts())
	}
}

func TestReport_RetriedStepCount_WithRetries(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)

	a.Attach(w)

	// Two flaky steps wired with retry config + one healthy step.
	// "a" fails once then succeeds (FailUntil=1, 2 attempts).
	// "b" fails twice then succeeds (FailUntil=2, 3 attempts).
	aStep := testhelpers.NewFlaky("a", 1)
	bStep := testhelpers.NewFlaky("b", 2)

	testhelpers.AddRetryStep(w, aStep, 2)
	testhelpers.AddRetryStep(w, bStep, 3)
	w.Add(flow.Step(testhelpers.NewSucceed("c")))

	testhelpers.RunWorkflow(t, a, w)

	r := a.Report()

	// Two steps retried (AttemptCount > 1).
	if r.RetriedStepCount() != 2 {
		t.Errorf("RetriedStepCount = %d, want 2", r.RetriedStepCount())
	}

	// "a" used 1 retry (2 attempts - 1), "b" used 2 retries (3 attempts - 1) → 3 total.
	if r.TotalRetryAttempts() != 3 {
		t.Errorf("TotalRetryAttempts = %d, want 3", r.TotalRetryAttempts())
	}

	if !r.HasWorkflowRetries() {
		t.Error("HasWorkflowRetries should be true when any step retried")
	}
}

func TestReport_TimedOutSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)

	a.Attach(w)

	w.Add(flow.Step(&timeoutStep{name: "slow"}))
	w.Add(flow.Step(testhelpers.NewSucceed("fast")))

	testhelpers.RunWorkflow(t, a, w)

	r := a.Report()

	timed := r.TimedOutSteps()
	if len(timed) != 1 {
		t.Errorf("TimedOutSteps length = %d, want 1", len(timed))
	}

	if len(timed) > 0 && timed[0].Name != "slow" {
		t.Errorf("TimedOutSteps[0].Name = %q, want %q", timed[0].Name, "slow")
	}

	if r.TimedOutStepCount() != 1 {
		t.Errorf("TimedOutStepCount = %d, want 1", r.TimedOutStepCount())
	}

	if !r.HasWorkflowTimeouts() {
		t.Error("HasWorkflowTimeouts should be true after a DeadlineExceeded failure")
	}
}

func TestReport_NoTimeouts(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)

	a.Attach(w)

	w.Add(flow.Step(testhelpers.NewSucceed("ok")))

	testhelpers.RunWorkflow(t, a, w)

	r := a.Report()

	if len(r.TimedOutSteps()) != 0 {
		t.Errorf("TimedOutSteps should be empty for non-timeout run, got %d", len(r.TimedOutSteps()))
	}

	if r.HasWorkflowTimeouts() {
		t.Error("HasWorkflowTimeouts should be false for non-timeout run")
	}
}

func TestReport_RetriedStepsCount_ManyFlakySteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)

	a.Attach(w)

	// 10 flaky steps, each failing once and succeeding on retry.
	// → 10 retried steps, 10 total retry attempts.
	names := []string{
		"f0", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9",
	}

	for _, n := range names {
		testhelpers.AddRetryStep(w, testhelpers.NewFlaky(n, 1), 2)
	}

	testhelpers.RunWorkflow(t, a, w)

	r := a.Report()

	if r.RetriedStepCount() != 10 {
		t.Errorf("RetriedStepCount = %d, want 10", r.RetriedStepCount())
	}

	if r.TotalRetryAttempts() != 10 {
		t.Errorf("TotalRetryAttempts = %d, want 10", r.TotalRetryAttempts())
	}

	// Sanity: no timeouts on retry-only runs.
	if r.HasWorkflowTimeouts() {
		t.Error("retry-only runs should not register timeouts")
	}
}

func TestReport_WorkflowHelpers_EmptyReport(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	r := a.Report()

	if r.RetriedStepCount() != 0 {
		t.Errorf("RetriedStepCount on empty report = %d, want 0", r.RetriedStepCount())
	}

	if r.TotalRetryAttempts() != 0 {
		t.Errorf("TotalRetryAttempts on empty report = %d, want 0", r.TotalRetryAttempts())
	}

	if r.HasWorkflowRetries() {
		t.Error("HasWorkflowRetries on empty report should be false")
	}

	if r.HasWorkflowTimeouts() {
		t.Error("HasWorkflowTimeouts on empty report should be false")
	}

	if r.TimedOutStepCount() != 0 {
		t.Errorf("TimedOutStepCount on empty report = %d, want 0", r.TimedOutStepCount())
	}
}

// Verify that a workflow with mixed timeouts and retries aggregates correctly.
func TestReport_MixedRetriesAndTimeouts(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)

	a.Attach(w)

	testhelpers.AddRetryStep(w, testhelpers.NewFlaky("retry-only", 1), 2)
	w.Add(flow.Step(&timeoutStep{name: "always-times-out"}))
	w.Add(flow.Step(testhelpers.NewSucceed("healthy")))

	testhelpers.RunWorkflow(t, a, w)

	r := a.Report()

	if r.RetriedStepCount() != 1 {
		t.Errorf("RetriedStepCount = %d, want 1", r.RetriedStepCount())
	}

	if r.TimedOutStepCount() != 1 {
		t.Errorf("TimedOutStepCount = %d, want 1", r.TimedOutStepCount())
	}

	if !r.HasWorkflowRetries() {
		t.Error("HasWorkflowRetries should be true")
	}

	if !r.HasWorkflowTimeouts() {
		t.Error("HasWorkflowTimeouts should be true")
	}
}
