package live_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/live"
)

func newTestServer(t *testing.T) *live.Server {
	t.Helper()

	hub := live.NewHub(nil)

	auditor, err := auditlog.New(auditlog.Config{
		Enabled: true,
		OnEvent: hub.OnEvent,
	})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	hub.SetAuditor(auditor)

	server := live.NewServer(hub, auditor, live.Config{})

	return server
}

func TestServer_DashboardHTML(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ctx := t.Context()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	for _, want := range []string{"<!DOCTYPE html>", "workflow-auditlog", "LIVE"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard HTML missing %q", want)
		}
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %s", ct)
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ctx := t.Context()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	for _, want := range []string{`"status"`, `"ok"`, `"clients"`, `"events"`} {
		if !strings.Contains(body, want) {
			t.Errorf("health response missing %q: %s", want, body)
		}
	}
}

func TestServer_ReportEndpoint(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ctx := t.Context()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/report", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, `"workflow_id"`) {
		t.Errorf("report response missing workflow_id: %s", body[:min(200, len(body))])
	}
}

func TestServer_NotFound(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ctx := t.Context()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestServer_NewConvenience(t *testing.T) {
	t.Parallel()

	server, auditor, err := live.New(auditlog.Config{
		WorkflowID: "test-workflow",
	}, live.Config{Addr: ":0"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if server == nil {
		t.Fatal("server is nil")
	}

	if auditor == nil {
		t.Fatal("auditor is nil")
	}

	if auditor.RunID() == "" {
		t.Error("auditor should have a RunID")
	}
}

// --- SSE Tests (use httptest.NewServer for real HTTP streaming) ---

func sseConnect(t *testing.T, url string) *bufio.Scanner {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect SSE: %v", err)
	}

	t.Cleanup(func() {
		cancel()

		_ = resp.Body.Close()
	})

	return bufio.NewScanner(resp.Body)
}

func skipSnapshot(scanner *bufio.Scanner) {
	for scanner.Scan() {
		if scanner.Text() == "" {
			break
		}
	}
}

func readSSEEvent(scanner *bufio.Scanner, eventName string) (string, bool) {
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: "+eventName) {
			scanner.Scan()

			dataLine := scanner.Text()
			data, found := strings.CutPrefix(dataLine, "data: ")

			if found {
				return data, true
			}
		}
	}

	return "", false
}

func readUntilStep(scanner *bufio.Scanner, stepName string) bool {
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), stepName) {
			return true
		}
	}

	return false
}

func TestServer_SSE_SnapshotOnConnect(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	server.OnEvent(auditlog.Event{
		Sequence:  1,
		StepRef:   auditlog.StepRef{Name: "step-a"},
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
	})

	ts := httptest.NewServer(server)
	defer ts.Close()

	scanner := sseConnect(t, ts.URL+"/api/events")

	data, found := readSSEEvent(scanner, "snapshot")
	if !found {
		t.Fatal("did not receive snapshot event")
	}

	if !strings.Contains(data, `"report"`) {
		t.Errorf("snapshot should contain report field: %s", data[:min(200, len(data))])
	}
}

func TestServer_SSE_LiveEventDelivery(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ts := httptest.NewServer(server)
	defer ts.Close()

	scanner := sseConnect(t, ts.URL+"/api/events")

	skipSnapshot(scanner)

	server.OnEvent(auditlog.Event{
		Sequence:  1,
		StepRef:   auditlog.StepRef{Name: "live-step"},
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
	})

	data, found := readSSEEvent(scanner, "event")
	if !found {
		t.Fatal("did not receive live event")
	}

	if !strings.Contains(data, "live-step") {
		t.Errorf("live event should contain live-step: %s", data)
	}
}

func TestServer_SSE_CompleteEvent(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ts := httptest.NewServer(server)
	defer ts.Close()

	scanner := sseConnect(t, ts.URL+"/api/events")

	skipSnapshot(scanner)

	server.SignalComplete()

	_, found := readSSEEvent(scanner, "complete")
	if !found {
		t.Fatal("did not receive complete event")
	}
}

func TestServer_SSE_FanOut(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ts := httptest.NewServer(server)
	defer ts.Close()

	scanner1 := sseConnect(t, ts.URL+"/api/events")
	scanner2 := sseConnect(t, ts.URL+"/api/events")

	skipSnapshot(scanner1)
	skipSnapshot(scanner2)

	server.OnEvent(auditlog.Event{
		Sequence:  1,
		StepRef:   auditlog.StepRef{Name: "fanout-step"},
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
	})

	if !readUntilStep(scanner1, "fanout-step") {
		t.Error("client 1 did not receive fanout event")
	}

	if !readUntilStep(scanner2, "fanout-step") {
		t.Error("client 2 did not receive fanout event")
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx := t.Context()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/health", nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET health: %v", err)
	}

	_ = resp.Body.Close()
}

func TestServer_ClientCount(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	if server.ClientCount() != 0 {
		t.Errorf("expected 0 clients initially, got %d", server.ClientCount())
	}
}

// --- Hub Unit Tests ---

func TestHub_SubscribeUnsubscribe(t *testing.T) {
	t.Parallel()

	hub := live.NewHub(nil)

	sub := hub.Subscribe()
	if sub == nil {
		t.Fatal("Subscribe returned nil")
	}

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.Unsubscribe(sub.ID())

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after unsubscribe, got %d", hub.ClientCount())
	}
}

func TestHub_OnEventDelivery(t *testing.T) {
	t.Parallel()

	hub := live.NewHub(nil)

	sub := hub.Subscribe()
	defer hub.Unsubscribe(sub.ID())

	evt := auditlog.Event{
		Sequence: 42,
		StepRef:  auditlog.StepRef{Name: "test"},
	}

	hub.OnEvent(evt)

	select {
	case received := <-sub.Events():
		if received.Sequence != 42 {
			t.Errorf("expected sequence 42, got %d", received.Sequence)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestHub_SignalComplete(t *testing.T) {
	t.Parallel()

	hub := live.NewHub(nil)

	sub := hub.Subscribe()
	defer hub.Unsubscribe(sub.ID())

	if hub.IsComplete() {
		t.Error("should not be complete initially")
	}

	hub.SignalComplete()

	if !hub.IsComplete() {
		t.Error("should be complete after SignalComplete")
	}

	select {
	case <-sub.Done():
	case <-time.After(time.Second):
		t.Fatal("done channel should be closed after SignalComplete")
	}
}

func TestHub_NonBlockingOnFullBuffer(t *testing.T) {
	t.Parallel()

	hub := live.NewHub(nil)

	sub := hub.Subscribe()
	defer hub.Unsubscribe(sub.ID())

	for i := range 300 {
		hub.OnEvent(auditlog.Event{Sequence: i})
	}

	done := make(chan struct{})

	go func() {
		hub.OnEvent(auditlog.Event{Sequence: 999})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("OnEvent blocked on full buffer")
	}
}
