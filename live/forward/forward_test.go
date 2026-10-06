package forward_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	return startFakeCollectorAtPath(t, path), path
}

func startFakeCollectorAtPath(t *testing.T, path string) *fakeCollector {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir socket dir: %v", err)
	}

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

	return collector
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
	return wfStatusEvent(runID, sequence, auditlog.StepStatusSucceeded)
}

func wfStatusEvent(runID string, sequence int, status auditlog.StepStatus) auditlog.Event {
	return auditlog.Event{
		StepRef:   auditlog.StepRef{Name: "build", StepType: "shell"},
		RunID:     auditlog.RunID(runID),
		Sequence:  sequence,
		Timestamp: time.Now().UTC(),
		EventType: auditlog.EventTypeAttemptEnd,
		Phase:     auditlog.PhaseAfter,
		Attempt:   1,
		Status:    status,
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

	// Default (empty target) with a dead conventional socket stays ARMED:
	// delivery starts whenever the socket answers (see the activation test).
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	fwd = forward.NewWithTarget("", "wf")
	if !fwd.Enabled() {
		t.Fatal("dead default socket must stay armed, not disable forwarding")
	}

	if err := fwd.Shutdown(context.Background()); err != nil {
		t.Fatalf("armed shutdown: %v", err)
	}
}

func TestForwarder_AutoTargetActivatesWhenSocketAppears(t *testing.T) {
	// SHORT temp runtime dir — unix socket paths must stay under the
	// kernel's 108-byte sun_path limit.
	runtimeDir, err := os.MkdirTemp("", "wfwd-xdg-*")
	if err != nil {
		t.Fatalf("temp runtime dir: %v", err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(runtimeDir) })

	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	fwd := forward.NewWithTarget("", "late-workflow")
	if !fwd.Enabled() {
		t.Fatal("empty target must arm the forwarder")
	}

	t.Cleanup(func() { _ = fwd.Shutdown(context.Background()) })

	// Events buffered while PapDashboard is still down…
	fwd.OnEvent(wfEvent("run-late", 1))

	// …and the collector appears later at the conventional socket.
	collector := startFakeCollectorAtPath(t, filepath.Join(runtimeDir, "papdashboard", "audit-runs.sock"))

	envelopes := collector.awaitEnvelopes(t, 1)
	if envelopes[0]["runId"] != "run-late" || envelopes[0]["sourceId"] != "late-workflow" {
		t.Fatalf("activation must flush buffered events: %+v", envelopes[0])
	}

	// Live events keep flowing through the now-active target; the explicit
	// marker rides the same batch (workflow semantics: one run, one flush).
	fwd.OnEvent(wfEvent("run-late", 2))
	fwd.Complete()

	all := collector.awaitEnvelopes(t, 2)
	last := all[len(all)-1]
	if last["complete"] != true || last["runId"] != "run-late" {
		t.Fatalf("explicit complete must flow after activation: %+v", last)
	}
}

func TestForwarder_HTTPTargetBearerKey(t *testing.T) {
	var gotAuth, gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotAuth = request.Header.Get("Authorization")
		gotPath = request.URL.Path

		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	t.Setenv("WORKFLOW_AUDITLOG_FORWARD_API_KEY", "secret-key")

	fwd := forward.NewWithTarget(server.URL, "http-workflow")
	if !fwd.Enabled() {
		t.Fatal("http target must enable the forwarder")
	}

	t.Cleanup(func() { _ = fwd.Shutdown(context.Background()) })

	fwd.OnEvent(wfEvent("run-http", 1))
	_ = fwd.Shutdown(context.Background())

	if gotAuth != "Bearer secret-key" {
		t.Fatalf("http target must send the bearer key, got %q", gotAuth)
	}

	if gotPath != "/events" {
		t.Fatalf("http target must POST the ingest path, got %q", gotPath)
	}
}

func TestForwarder_FanOutToUnixAndHTTP(t *testing.T) {
	collector, socketPath := startFakeCollector(t)

	httpSink := make(chan string, 8)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		httpSink <- string(body)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	fwd := forward.NewWithTarget("unix://"+socketPath+","+server.URL, "fan-workflow")
	if !fwd.Enabled() {
		t.Fatal("fan-out spec must enable the forwarder")
	}

	t.Cleanup(func() { _ = fwd.Shutdown(context.Background()) })

	fwd.OnEvent(wfEvent("run-fan", 1))
	_ = fwd.Shutdown(context.Background())

	envelopes := collector.awaitEnvelopes(t, 1)
	if envelopes[0]["runId"] != "run-fan" {
		t.Fatalf("unix leg must receive the batch: %+v", envelopes[0])
	}

	select {
	case body := <-httpSink:
		if !strings.Contains(body, `"runId":"run-fan"`) {
			t.Fatalf("http leg must receive the batch: %s", body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("http leg received nothing")
	}
}

func TestForwarder_PostFailuresCountedAndLogged(t *testing.T) {
	var logBuf bytes.Buffer

	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	fwd := forward.NewWithTarget(server.URL, "flaky-workflow")

	t.Cleanup(func() { _ = fwd.Shutdown(context.Background()) })

	for i := range 3 {
		fwd.OnEvent(wfEvent("run-flaky", i+1))
		time.Sleep(300 * time.Millisecond) // let flushes hit the failing target
	}

	if fwd.Failed() == 0 {
		t.Fatal("failed POSTs must be counted")
	}

	transitionLogs := strings.Count(logBuf.String(), "delivery failing")
	if transitionLogs != 1 {
		t.Fatalf("failure transition must be logged exactly once, got %d: %s", transitionLogs, logBuf.String())
	}
}

func TestForwarder_BatchMaxChunksEnvelopes(t *testing.T) {
	collector, socketPath := startFakeCollector(t)

	t.Setenv("WORKFLOW_AUDITLOG_FORWARD_BATCH_MAX", "2")

	fwd := forward.NewWithTarget("unix://"+socketPath, "chunk-workflow")

	for i := range 5 {
		fwd.OnEvent(wfEvent("run-chunk", i+1))
	}

	_ = fwd.Shutdown(context.Background())

	envelopes := collector.awaitEnvelopes(t, 3)
	if len(envelopes) < 3 {
		t.Fatalf("batch max 2 over 5 events must chunk into 3 envelopes, got %d", len(envelopes))
	}

	total := 0
	for _, envelope := range envelopes {
		events, _ := envelope["events"].([]any)
		total += len(events)
	}

	if total != 5 {
		t.Fatalf("chunks must carry every event exactly once, got %d of 5", total)
	}
}

func TestForwarder_CompleteOnErrorDerivation(t *testing.T) {
	collector, socketPath := startFakeCollector(t)

	t.Setenv("WORKFLOW_AUDITLOG_FORWARD_COMPLETE_ON_ERROR", "1")

	fwd := forward.NewWithTarget("unix://"+socketPath, "crash-workflow")

	t.Cleanup(func() { _ = fwd.Shutdown(context.Background()) })

	// A terminal failed attempt with no SignalComplete must still complete
	// the run when the derivation knob is on.
	fwd.OnEvent(wfEvent("run-crash", 1))
	fwd.OnEvent(wfStatusEvent("run-crash", 2, auditlog.StepStatusFailed))

	envelopes := collector.awaitEnvelopes(t, 1)
	if envelopes[0]["complete"] != true {
		t.Fatalf("terminal error status must derive completion: %+v", envelopes[0])
	}
}
