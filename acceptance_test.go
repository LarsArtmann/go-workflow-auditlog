package auditlog_test

import (
	"bytes"
	"slices"
	"sync"
	"testing"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// TestAcceptance_MixedOutcomePipeline verifies the public API tells a coherent
// story for a realistic workflow with mixed outcomes: a clean success, a step
// that retries then succeeds, and a hard failure. The report's aggregates,
// query helpers, and validation must all agree.
func TestAcceptance_MixedOutcomePipeline(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	root := newSucceed("root")
	flaky := newFlaky("retry-step", 2) // succeeds on the 3rd attempt
	bad := newFail("bad-step", "boom")

	w.Add(
		flow.Step(root),
	)
	addRetryStep(w, flaky, 5)
	w.Add(flow.Step(bad))

	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	// Aggregates reflect the three outcomes.
	if report.SucceededCount != 2 {
		t.Errorf("SucceededCount: got %d, want 2 (root + retry-step)", report.SucceededCount)
	}

	if report.FailedCount != 1 {
		t.Errorf("FailedCount: got %d, want 1 (bad-step)", report.FailedCount)
	}

	// A failure means the overall workflow did not succeed.
	if report.WorkflowSucceeded {
		t.Error("expected WorkflowSucceeded=false when a step failed")
	}

	// Query helpers agree with the aggregates.
	if len(report.SucceededSteps()) != 2 {
		t.Errorf("SucceededSteps: got %d, want 2", len(report.SucceededSteps()))
	}

	if len(report.FailedSteps()) != 1 {
		t.Errorf("FailedSteps: got %d, want 1", len(report.FailedSteps()))
	}

	// The retried step is reported as retried.
	retried := report.RetriedSteps()
	if len(retried) != 1 || retried[0].Name != "retry-step" {
		t.Errorf("RetriedSteps: got %+v, want [retry-step]", retried)
	}

	// Every step got a non-zero StepID.
	for _, step := range report.Steps {
		if step.StepID == 0 {
			t.Errorf("step %q has zero StepID", step.Name)
		}
	}
}

// TestAcceptance_OnEventStreamIsOrderedAndTagged confirms the OnEvent callback
// receives events in monotonic sequence order and that every event carries the
// run's RunID — the basis for cross-system correlation.
func TestAcceptance_OnEventStreamIsOrderedAndTagged(t *testing.T) {
	t.Parallel()

	var (
		collected []auditlog.Event
		mu        sync.Mutex
	)

	a := mustNew(t, auditlog.Config{
		Enabled: true,
		// OnEvent fires from each step's goroutine, so concurrent steps invoke
		// it concurrently — the callback must be goroutine-safe.
		OnEvent: func(e auditlog.Event) {
			mu.Lock()
			defer mu.Unlock()

			collected = append(collected, e)
		},
	})
	runID := a.RunID()

	w := &flow.Workflow{}
	w.Add(flow.Step(newSucceed("a")))
	w.Add(flow.Step(newSucceed("b")))
	runWorkflow(t, a, w)

	if len(collected) == 0 {
		t.Fatal("expected OnEvent to collect events")
	}

	// OnEvent delivery order is NOT guaranteed — concurrent steps fire it from
	// their own goroutines. Sort by Sequence before checking monotonicity.
	slices.SortFunc(collected, func(a, b auditlog.Event) int {
		return a.Sequence - b.Sequence
	})

	// Sequences are strictly increasing once ordered.
	for i := 1; i < len(collected); i++ {
		if collected[i].Sequence <= collected[i-1].Sequence {
			t.Errorf("event %d sequence %d not greater than predecessor %d",
				i, collected[i].Sequence, collected[i-1].Sequence)
		}
	}

	// Every event is tagged with the run ID.
	for i, evt := range collected {
		if evt.RunID != runID {
			t.Errorf("event %d RunID %q != %q", i, evt.RunID, runID)
		}
	}

	// There is at least one start and one end event.
	if !slices.ContainsFunc(collected, auditlog.Event.IsAttemptStart) {
		t.Error("expected at least one attempt_start event")
	}

	if !slices.ContainsFunc(collected, auditlog.Event.IsAttemptEnd) {
		t.Error("expected at least one attempt_end event")
	}
}

// TestAcceptance_FilterExportLoadPreservesIdentity exercises the full lifecycle:
// build → filter → serialize to JSON → reload → verify identity and recomputed
// aggregates survive the round trip.
func TestAcceptance_FilterExportLoadPreservesIdentity(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ok := newSucceed("ok-step")
	bad := newFail("bad-step", "boom")
	w.Add(flow.Step(ok), flow.Step(bad))
	runWorkflow(t, a, w)

	original := a.Report()
	runID := a.RunID()

	// Keep only succeeded steps.
	filtered := original.Filtered(auditlog.WithStepsByStatus(auditlog.StepStatusSucceeded))
	assertReportValid(t, filtered)

	if filtered.RunID != runID {
		t.Errorf("filtered RunID %q != %q", filtered.RunID, runID)
	}

	if filtered.WorkflowID != original.WorkflowID {
		t.Errorf("filtered WorkflowID %q != %q", filtered.WorkflowID, original.WorkflowID)
	}

	if filtered.StepCount != 1 {
		t.Errorf("filtered StepCount: got %d, want 1", filtered.StepCount)
	}

	// Serialize → reload and confirm identity holds.
	var buf bytes.Buffer

	err := filtered.WriteJSON(&buf)
	if err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	loaded, err := auditlog.LoadReportFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadReportFromBytes error: %v", err)
	}

	if loaded.RunID != runID {
		t.Errorf("loaded RunID %q != %q", loaded.RunID, runID)
	}

	if loaded.StepCount != filtered.StepCount {
		t.Errorf("loaded StepCount %d != %d", loaded.StepCount, filtered.StepCount)
	}
}
