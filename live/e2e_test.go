package live_test

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/live"
)

// e2eStep is a minimal workflow step for end-to-end testing.
type e2eStep struct {
	name  string
	delay time.Duration
	fail  bool
}

func (s *e2eStep) Do(ctx context.Context) error {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if s.fail {
		return errE2EStepFailed
	}

	return nil
}

func (s *e2eStep) String() string { return s.name }

var errE2EStepFailed = e2eStepError("e2e step failed")

type e2eStepError string

func (e e2eStepError) Error() string { return string(e) }

// readAllSSEEvents reads all "event" messages until "complete" arrives.
// Returns the event data payloads and whether complete was received.
func readAllSSEEvents(scanner *bufio.Scanner) ([]string, bool) {
	var events []string

	gotComplete := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: event") {
			scanner.Scan() // data line

			dataLine := scanner.Text()
			if data, ok := strings.CutPrefix(dataLine, "data: "); ok {
				events = append(events, data)
			}
		} else if strings.HasPrefix(line, "event: complete") {
			gotComplete = true

			break
		}
	}

	return events, gotComplete
}

// TestServer_SSE_EndToEnd runs a real workflow through the live server,
// connects an SSE client, and verifies that streamed events match the
// auditor's internal event stream.
func TestServer_SSE_EndToEnd(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{
		WorkflowID: "e2e-test-workflow",
		Enabled:    true,
		OnEvent:    hub.OnEvent,
	})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{})

	// Build a 3-step linear workflow: fetch → validate → save
	w := &flow.Workflow{}

	fetch := &e2eStep{name: "fetch", delay: 5 * time.Millisecond}
	validate := &e2eStep{name: "validate", delay: 5 * time.Millisecond}
	save := &e2eStep{name: "save", delay: 5 * time.Millisecond}

	w.Add(flow.Step(fetch))
	w.Add(flow.Step(validate).DependsOn(fetch))
	w.Add(flow.Step(save).DependsOn(validate))

	auditor.Attach(w)
	auditor.CaptureDAG(w)

	ts := httptest.NewServer(server)
	defer ts.Close()

	// Connect SSE client before running the workflow
	scanner, closeSSE := sseConnect(t, ts.URL+"/api/events")
	defer closeSSE()

	skipSnapshot(scanner)

	// Run the workflow synchronously
	ctx := t.Context()

	runErr := w.Do(ctx)
	if runErr != nil {
		t.Fatalf("workflow Do failed: %v", runErr)
	}

	auditor.Snapshot(w)

	// Brief pause to let buffered events flush through the SSE handler
	// before the complete signal fires (avoids select-race on done vs ch).
	time.Sleep(50 * time.Millisecond)

	server.SignalComplete()

	sseEvents, gotComplete := readAllSSEEvents(scanner)
	if !gotComplete {
		t.Fatal("did not receive complete event")
	}

	// Verify all expected step names appear in SSE events
	expectedSteps := map[string]bool{"fetch": false, "validate": false, "save": false}

	for _, data := range sseEvents {
		for name := range expectedSteps {
			if strings.Contains(data, name) {
				expectedSteps[name] = true
			}
		}
	}

	for name, found := range expectedSteps {
		if !found {
			t.Errorf("step %q not found in SSE events", name)
		}
	}

	// Verify SSE events match auditor events by sequence + step name
	auditorEvents := auditor.Events()

	if len(sseEvents) == 0 {
		t.Fatal("expected SSE events, got none")
	}

	// Build a set of auditor event sequences for lookup
	auditorBySeq := make(map[int]auditlog.Event, len(auditorEvents))
	for _, ae := range auditorEvents {
		auditorBySeq[ae.Sequence] = ae
	}

	for _, data := range sseEvents {
		var sseEvt auditlog.Event

		jsonErr := json.Unmarshal([]byte(data), &sseEvt)
		if jsonErr != nil {
			t.Errorf("unmarshal SSE event: %v (data: %s)", jsonErr, data[:min(100, len(data))])

			continue
		}

		ae, ok := auditorBySeq[sseEvt.Sequence]
		if !ok {
			t.Errorf("SSE event seq %d not found in auditor events", sseEvt.Sequence)

			continue
		}

		if ae.Name != sseEvt.Name {
			t.Errorf("event seq %d: step name mismatch: SSE %q vs auditor %q",
				sseEvt.Sequence, sseEvt.Name, ae.Name)
		}

		if ae.EventType != sseEvt.EventType {
			t.Errorf("event seq %d: type mismatch: SSE %q vs auditor %q",
				sseEvt.Sequence, sseEvt.EventType, ae.EventType)
		}
	}

	// Each step generates attempt_start + attempt_end = 2 events minimum
	minExpected := len(expectedSteps) * 2
	if len(sseEvents) < minExpected {
		t.Errorf("expected at least %d SSE events, got %d", minExpected, len(sseEvents))
	}
}

// TestServer_SSE_EndToEnd_FailingWorkflow verifies that failing steps
// produce error fields in the SSE event stream.
func TestServer_SSE_EndToEnd_FailingWorkflow(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{
		WorkflowID: "e2e-fail-workflow",
		Enabled:    true,
		OnEvent:    hub.OnEvent,
	})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{})

	w := &flow.Workflow{}

	failStep := &e2eStep{name: "doomed", fail: true}

	w.Add(flow.Step(failStep))

	auditor.Attach(w)
	auditor.CaptureDAG(w)

	ts := httptest.NewServer(server)
	defer ts.Close()

	scanner, closeSSE := sseConnect(t, ts.URL+"/api/events")
	defer closeSSE()

	skipSnapshot(scanner)

	ctx := t.Context()
	_ = w.Do(ctx) // expected to fail

	auditor.Snapshot(w)

	time.Sleep(50 * time.Millisecond)

	server.SignalComplete()

	sseEvents, gotComplete := readAllSSEEvents(scanner)
	if !gotComplete {
		t.Fatal("did not receive complete event")
	}

	if len(sseEvents) == 0 {
		t.Fatal("expected SSE events for failing workflow")
	}

	// At least one event should contain an error field
	foundError := false

	for _, data := range sseEvents {
		if strings.Contains(data, `"error"`) {
			foundError = true

			break
		}
	}

	if !foundError {
		t.Error("no error field found in SSE events for failing step")
	}
}
