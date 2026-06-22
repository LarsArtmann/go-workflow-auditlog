package auditlog_test

import (
	"bytes"
	"strings"
	"testing"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestReadEvents_RoundTrip(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSucceed("rt-step-1")
	s2 := newFail("rt-step-2", "boom")
	addParallelSteps(w, s1, s2)
	runWorkflow(t, a, w)

	// Export events as NDJSON.
	var buf bytes.Buffer

	err := a.WriteNDJSON(&buf)
	if err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}

	// Read them back.
	events, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	originalEvents := a.Events()
	if len(events) != len(originalEvents) {
		t.Fatalf("expected %d events, got %d", len(originalEvents), len(events))
	}

	// Verify the events match.
	for i, evt := range events {
		if evt.Name != originalEvents[i].Name {
			t.Errorf("event %d: expected name %q, got %q", i, originalEvents[i].Name, evt.Name)
		}

		if evt.Sequence != originalEvents[i].Sequence {
			t.Errorf("event %d: expected seq %d, got %d", i, originalEvents[i].Sequence, evt.Sequence)
		}

		if evt.EventType != originalEvents[i].EventType {
			t.Errorf("event %d: expected type %s, got %s", i, originalEvents[i].EventType, evt.EventType)
		}
	}
}

func TestReadEvents_InvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"blank-only", "\n\n\n"},
		{"invalid-JSON", `{"bad": json}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := auditlog.ReadEvents(strings.NewReader(tc.input))
			if err == nil {
				t.Fatalf("expected error for %s input", tc.name)
			}
		})
	}
}

// TestReadEvents_LineNumberInError verifies that the error message reports the
// actual line number (counting blank lines), not the event count. This is a
// regression test for a prior bug where blank lines were skipped in the count.
func TestReadEvents_LineNumberInError(t *testing.T) {
	t.Parallel()

	// Two blank lines, then a malformed event on line 3.
	input := "\n\n{bad json}\n"

	_, err := auditlog.ReadEvents(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for malformed input")
	}

	assertContains(t, err.Error(), "line 3", "expected error to mention 'line 3', got "+err.Error())
}

func TestReadEvents_SkipsBlankLines(t *testing.T) {
	t.Parallel()

	input := `{"sequence":1,"event_type":"attempt_start","step_name":"a"}` + "\n\n" +
		`{"sequence":2,"event_type":"attempt_end","step_name":"a"}` + "\n"

	events, err := auditlog.ReadEvents(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events (blank line skipped), got %d", len(events))
	}
}

func TestReplayEvents_BasicReconstruction(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSucceed("replay-step")
	w.Add(flow.Step(s1))
	runWorkflow(t, a, w)

	events := a.Events()

	report, err := auditlog.ReplayEvents(events)
	if err != nil {
		t.Fatalf("ReplayEvents: %v", err)
	}

	if !report.Reconstructed {
		t.Error("expected Reconstructed=true")
	}

	if report.StepCount != 1 {
		t.Fatalf("expected 1 step, got %d", report.StepCount)
	}

	step := report.StepByName("replay-step")
	if step == nil {
		t.Fatal("expected to find replay-step")
	}

	if step.Status != auditlog.StepStatusSucceeded {
		t.Errorf("expected succeeded, got %s", step.Status)
	}

	if step.AttemptCount != 1 {
		t.Errorf("expected 1 attempt, got %d", step.AttemptCount)
	}
}

func TestReplayEvents_WithFailure(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newFail("failed", "boom")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	report, err := auditlog.ReplayEvents(a.Events())
	if err != nil {
		t.Fatalf("ReplayEvents: %v", err)
	}

	step := report.StepByName("failed")
	if step.Status != auditlog.StepStatusFailed {
		t.Errorf("expected failed, got %s", step.Status)
	}

	if step.Error == nil || *step.Error != "boom" {
		t.Errorf("expected error 'boom', got %v", step.Error)
	}
}

func TestReplayEvents_NoEvents(t *testing.T) {
	t.Parallel()

	_, err := auditlog.ReplayEvents(nil)
	if err == nil {
		t.Fatal("expected error for no events")
	}
}

func TestReplayEvents_PreservesEventCount(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSucceed("count-step")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	report, err := auditlog.ReplayEvents(a.Events())
	if err != nil {
		t.Fatalf("ReplayEvents: %v", err)
	}

	if report.EventCount != a.EventsCount() {
		t.Errorf("expected %d events, got %d", a.EventsCount(), report.EventCount)
	}
}

func TestReplayEvents_FullRoundTrip(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSucceed("rt-1")
	s2 := newFail("rt-2", "err")
	addDependentStep(w, s1, s2)
	runWorkflow(t, a, w)

	// Export NDJSON.
	var buf bytes.Buffer

	_ = a.WriteNDJSON(&buf)

	// Read back + replay.
	events, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	report, err := auditlog.ReplayEvents(events)
	if err != nil {
		t.Fatalf("ReplayEvents: %v", err)
	}

	// Verify the reconstructed report.
	if report.StepCount != 2 {
		t.Errorf("expected 2 steps, got %d", report.StepCount)
	}

	assertFailedCount(t, report, 1)

	if !report.Reconstructed {
		t.Error("expected Reconstructed=true")
	}
}
