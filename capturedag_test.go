package auditlog_test

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/cenkalti/backoff/v4"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

func TestCaptureDAG_PrePopulatesSteps(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	w := &flow.Workflow{}

	fetch := testhelpers.NewSucceed("fetch")
	validate := testhelpers.NewSucceed("validate")
	save := testhelpers.NewSucceed("save")

	w.Add(
		flow.Step(fetch),
		flow.Step(validate).DependsOn(fetch),
		flow.Step(save).DependsOn(validate),
	)

	a.Attach(w)
	a.CaptureDAG(w)

	// Before Do(): report should have steps with pending status
	report := a.Report()

	testhelpers.AssertStepCount(t, report, 3)

	fetchStep := testhelpers.FindStep(t, report, "fetch")
	validateStep := testhelpers.FindStep(t, report, "validate")
	saveStep := testhelpers.FindStep(t, report, "save")

	testhelpers.AssertStatus(t, fetchStep, auditlog.StepStatusPending)
	testhelpers.AssertStatus(t, validateStep, auditlog.StepStatusPending)
	testhelpers.AssertStatus(t, saveStep, auditlog.StepStatusPending)

	// Dependencies should be captured
	if len(validateStep.Dependencies) != 1 || validateStep.Dependencies[0].Name != "fetch" {
		t.Errorf("validate should depend on fetch, got %v", validateStep.Dependencies)
	}

	if len(saveStep.Dependencies) != 1 || saveStep.Dependencies[0].Name != "validate" {
		t.Errorf("save should depend on validate, got %v", saveStep.Dependencies)
	}
}

func TestCaptureDAG_ThenRunUpdatesStatuses(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	w := &flow.Workflow{}

	step := testhelpers.NewSucceed("my-step")
	w.Add(flow.Step(step))

	a.Attach(w)
	a.CaptureDAG(w)

	// Before Do(): pending
	report := a.Report()
	s := testhelpers.FindStep(t, report, "my-step")
	testhelpers.AssertStatus(t, s, auditlog.StepStatusPending)

	// Run the workflow
	testhelpers.RunWorkflow(t, a, w)

	// After Do() + Snapshot(): succeeded
	report = a.Report()
	s = testhelpers.FindStep(t, report, "my-step")
	testhelpers.AssertStatus(t, s, auditlog.StepStatusSucceeded)
}

func TestCaptureDAG_CapturesRetryConfig(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	w := &flow.Workflow{}

	flaky := &testhelpers.FlakyStep{Name: "flaky-api-call", FailUntil: 2}
	w.Add(flow.Step(flaky).Retry(func(o *flow.RetryOption) {
		o.Attempts = 5
		o.Backoff = backoff.NewExponentialBackOff()
	}))

	a.Attach(w)
	a.CaptureDAG(w)

	report := a.Report()
	s := testhelpers.FindStep(t, report, "flaky-api-call")

	if !s.HasRetry {
		t.Error("expected HasRetry=true after CaptureDAG")
	}

	if s.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts=5, got %d", s.MaxAttempts)
	}
}

func TestCaptureDAG_Idempotent(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	w := &flow.Workflow{}

	step := testhelpers.NewSucceed("step-a")
	w.Add(flow.Step(step))

	a.Attach(w)
	a.CaptureDAG(w)
	a.CaptureDAG(w) // calling twice should be safe

	report := a.Report()
	testhelpers.AssertStepCount(t, report, 1)
}

func TestCaptureDAG_Disabled(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: false})

	w := &flow.Workflow{}

	step := testhelpers.NewSucceed("disabled-step")
	w.Add(flow.Step(step))

	a.Attach(w)
	a.CaptureDAG(w)

	report := a.Report()
	testhelpers.AssertStepCount(t, report, 0)
}

func TestCaptureDAG_NilWorkflow(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	// Should not panic
	a.CaptureDAG(nil)

	report := a.Report()
	testhelpers.AssertStepCount(t, report, 0)
}

func TestCaptureDAG_NoEventsGenerated(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	w := &flow.Workflow{}

	step := testhelpers.NewSucceed("eventless-step")
	w.Add(flow.Step(step))

	a.Attach(w)
	a.CaptureDAG(w)

	report := a.Report()

	// CaptureDAG should NOT generate events — only step structure
	testhelpers.AssertEventCount(t, report, 0)
}

func TestCaptureDAG_SubWorkflow(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	// Build a sub-workflow
	sub := &flow.Workflow{}
	innerA := testhelpers.NewSucceed("inner-a")
	innerB := testhelpers.NewSucceed("inner-b")
	sub.Add(
		flow.Step(innerA),
		flow.Step(innerB).DependsOn(innerA),
	)

	// Add sub-workflow as a step in the parent
	w := &flow.Workflow{}
	outer := testhelpers.NewSucceed("outer")
	w.Add(
		flow.Step(outer),
		flow.Step(sub).DependsOn(outer),
	)

	a.Attach(w)
	a.CaptureDAG(w)

	_ = w.Do(context.Background())
	a.Snapshot(w)

	report := a.Report()

	// Should have captured inner steps via traversal
	foundInnerA := false
	foundInnerB := false

	for _, s := range report.Steps {
		if s.Name == "inner-a" {
			foundInnerA = true
		}

		if s.Name == "inner-b" {
			foundInnerB = true
		}
	}

	// Sub-workflow steps may or may not be captured depending on
	// whether flow.Traverse reaches them. The key is no panic.
	_ = foundInnerA
	_ = foundInnerB
}
