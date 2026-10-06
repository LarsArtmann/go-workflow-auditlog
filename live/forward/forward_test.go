package forward_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/live/forward"
)

// fakeCollector accepts ingest POSTs on a unix socket and records them.
type fakeCollector struct {
	mu        sync.Mutex
	envelopes []map[string]any

	listener net.Listener
	server   *http.Server
	wg       sync.WaitGroup
}

func startFakeCollector(t *testing.T) (*fakeCollector, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "audit-runs.sock")

	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}

	collector := &fakeCollector{listener: listener}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", func(writer http.ResponseWriter, request *http.Request) {
		var envelope map[string]any
		if err := json.NewDecoder(request.Body).Decode(&envelope); err != nil {
			writer.WriteHeader(http.StatusBadRequest)

			return
		}

		collector.mu.Lock()
		collector.envelopes = append(collector.envelopes, envelope)
		collector.mu.Unlock()

		writer.WriteHeader(http.StatusOK)
	})

	collector.server = &http.Server{Handler: mux, ReadHeaderTimeout: time.Second} //nolint:exhaustruct // test double
	collector.wg.Add(1)

	go func() {
		defer collector.wg.Done()
		_ = collector.server.Serve(listener)
	}()

	t.Cleanup(func() {
		_ = collector.server.Close()
		collector.wg.Wait()
	})

	return collector, path
}

func (c *fakeCollector) awaitEnvelopes(t *testing.T, want int) []map[string]any {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		count := len(c.envelopes)
		c.mu.Unlock()

		if count >= want {
			c.mu.Lock()
			defer c.mu.Unlock()

			return append([]map[string]any(nil), c.envelopes...)
		}

		time.Sleep(25 * time.Millisecond)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	t.Fatalf("collector saw %d envelopes, want %d", len(c.envelopes), want)

	return nil
}

func wfEvent(runID string, sequence int) auditlog.Event {
	return auditlog.Event{
		StepRef:   auditlog.StepRef{Name: "build", StepType: "shell"},
		RunID:     auditlog.RunID(runID),
		Sequence:  sequence,
		Timestamp: time.Now().UTC(),
		EventType: auditlog.EventTypeAttemptEnd,
		Phase:     auditlog.PhaseAfter,
		Attempt:   1,
		Status:    auditlog.StepStatusSucceeded,
	}
}

func TestForwarder_UnixRoundTrip(t *testing.T) {
	collector, path := startFakeCollector(t)

	fwd := forward.NewWithTarget("unix://"+path, "deploy-workflow")
	if !fwd.Enabled() {
		t.Fatal("explicit unix target must enable the forwarder")
	}

	t.Cleanup(func() { _ = fwd.Shutdown(context.Background()) })

	fwd.OnEvent(wfEvent("run-wf-1", 1))
	fwd.OnEvent(wfEvent("run-wf-1", 2))

	envelopes := collector.awaitEnvelopes(t, 1)
	first := envelopes[0]

	if first["kind"] != "workflow" || first["sourceId"] != "deploy-workflow" || first["runId"] != "run-wf-1" {
		t.Fatalf("envelope identity wrong: %+v", first)
	}

	if first["complete"] != false {
		t.Fatalf("batch without Complete() must not carry completion: %+v", first)
	}

	events, ok := first["events"].([]any)
	if !ok || len(events) != 2 {
		t.Fatalf("batch must carry 2 events: %+v", first["events"])
	}

	// Events ride the upstream snake_case tags verbatim.
	firstEvent, _ := events[0].(map[string]any)
	if firstEvent["step_name"] != "build" || firstEvent["event_type"] != "attempt_end" {
		t.Fatalf("event JSON must keep the auditlog wire tags: %+v", firstEvent)
	}
}

func TestForwarder_ExplicitCompleteRidesLastBatch(t *testing.T) {
	collector, path := startFakeCollector(t)

	fwd := forward.NewWithTarget("unix://"+path, "deploy-workflow")

	t.Cleanup(func() { _ = fwd.Shutdown(context.Background()) })

	fwd.OnEvent(wfEvent("run-wf-2", 1))
	fwd.Complete()

	envelopes := collector.awaitEnvelopes(t, 1)
	if envelopes[0]["complete"] != true || envelopes[0]["runId"] != "run-wf-2" {
		t.Fatalf("Complete() must mark the last seen run's batch: %+v", envelopes[0])
	}
}

func TestForwarder_OffModes(t *testing.T) {
	fwd := forward.NewWithTarget("off", "wf")
	if fwd.Enabled() {
		t.Fatal("off target must disable forwarding")
	}

	// All methods must be safe no-ops.
	fwd.OnEvent(wfEvent("run-x", 1))
	fwd.Complete()

	if err := fwd.Shutdown(context.Background()); err != nil {
		t.Fatalf("disabled shutdown: %v", err)
	}
}
