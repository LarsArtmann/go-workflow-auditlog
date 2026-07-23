package auditlog_test

import (
	"bytes"
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

// TestReplayEvents_RoundTripFromWorkflow verifies the full export → read →
// replay cycle starting from a real workflow run, checking that the replayed
// report matches the original on key fields.
func TestReplayEvents_RoundTripFromWorkflow(t *testing.T) {
	t.Parallel()

	// Run a real workflow to get a report with events.
	a := testhelpers.RunSingleSucceed(t, "roundtrip-step")
	original := a.Report()

	if original.EventCount == 0 {
		t.Fatal("expected events in original report")
	}

	// Export events as NDJSON.
	var buf bytes.Buffer

	err := original.WriteNDJSON(&buf)
	if err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}

	// Read events back.
	events, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != original.EventCount {
		t.Fatalf("event count mismatch: original=%d read=%d", original.EventCount, len(events))
	}

	// Replay into a report.
	replayed, err := auditlog.ReplayEvents(events)
	if err != nil {
		t.Fatalf("ReplayEvents: %v", err)
	}

	// Verify key fields match.
	if replayed.EventCount != original.EventCount {
		t.Errorf("EventCount: original=%d replayed=%d", original.EventCount, replayed.EventCount)
	}

	if replayed.StepCount != original.StepCount {
		t.Errorf("StepCount: original=%d replayed=%d", original.StepCount, replayed.StepCount)
	}

	if replayed.SucceededCount != original.SucceededCount {
		t.Errorf("SucceededCount: original=%d replayed=%d", original.SucceededCount, replayed.SucceededCount)
	}

	if replayed.WorkflowSucceeded != original.WorkflowSucceeded {
		t.Errorf("WorkflowSucceeded: original=%v replayed=%v", original.WorkflowSucceeded, replayed.WorkflowSucceeded)
	}

	// Verify step names match.
	if len(replayed.Steps) > 0 && replayed.Steps[0].Name != original.Steps[0].Name {
		t.Errorf("first step name: original=%q replayed=%q",
			original.Steps[0].Name, replayed.Steps[0].Name)
	}

	// Replayed report should be internally consistent.
	err = replayed.Validate()
	if err != nil {
		t.Errorf("replayed report failed validation: %v", err)
	}

	// Replayed report should be marked as reconstructed.
	if !replayed.Reconstructed {
		t.Error("expected Reconstructed=true for replayed report")
	}
}

// TestReplayEvents_RoundTripMultipleSteps verifies the round-trip with a
// multi-step workflow including dependencies.
func TestReplayEvents_RoundTripMultipleSteps(t *testing.T) {
	t.Parallel()

	a, _ := auditlog.New(auditlog.Config{Enabled: true, WorkflowID: "multi-roundtrip"})

	fetch := testhelpers.NewSucceed("multi-fetch")
	save := testhelpers.NewSucceed("multi-save")

	w := &flow.Workflow{}
	w.Add(
		flow.Step(fetch),
		flow.Step(save).DependsOn(fetch),
	)

	a.Attach(w)
	_ = w.Do(context.Background())
	a.Snapshot(w)

	original := a.Report()

	if original.StepCount != 2 {
		t.Fatalf("expected 2 steps, got %d", original.StepCount)
	}

	var buf bytes.Buffer

	err := original.WriteNDJSON(&buf)
	if err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}

	events, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	replayed, err := auditlog.ReplayEvents(events)
	if err != nil {
		t.Fatalf("ReplayEvents: %v", err)
	}

	if replayed.StepCount != 2 {
		t.Errorf("expected 2 replayed steps, got %d", replayed.StepCount)
	}

	// Verify both step names survived.
	names := make(map[string]bool)
	for _, s := range replayed.Steps {
		names[s.Name] = true
	}

	if !names["multi-fetch"] || !names["multi-save"] {
		t.Errorf("expected steps multi-fetch and multi-save, got %v", names)
	}
}
