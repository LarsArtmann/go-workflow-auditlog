package live_test

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/live"
)

func newTestServer(t *testing.T) *live.Server {
	t.Helper()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{
		Enabled: true,
		OnEvent: hub.OnEvent,
	})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{})

	return server
}

func newTestServerWithCORS(t *testing.T, corsOrigin string) *live.Server {
	t.Helper()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{
		Enabled: true,
		OnEvent: hub.OnEvent,
	})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	return live.NewServer(hub, auditor, live.Config{
		CORSAllowedOrigins: corsOrigin,
	})
}

// serveTestRequest runs a GET path against a freshly-built test server and
// returns the response recorder. Centralizes the (server + ctx + req + rec +
// ServeHTTP) boilerplate shared by every endpoint test in this file.
func serveTestRequest(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()

	server := newTestServer(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	return rec
}

func TestServer_DashboardHTML(t *testing.T) {
	t.Parallel()

	rec := serveTestRequest(t, "/")

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

// TestServer_DashboardHTML_Accessibility validates that the live dashboard HTML
// contains keyboard-navigation and accessibility landmarks added during the
// keyboard navigation improvement work.
func TestServer_DashboardHTML_Accessibility(t *testing.T) {
	t.Parallel()

	rec := serveTestRequest(t, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	for _, want := range []string{
		"class=\"skip-link\"",
		"id=\"main-content\"",
		"role=\"main\"",
		"role=\"banner\"",
		"role=\"navigation\"",
		"aria-live=\"polite\"",
		"role=\"dialog\"",
		"id=\"keyboard-help\"",
		"tabindex=\"0\"",
		"role=\"button\"",
		"aria-sort=\"ascending\"",
		"id=\"help-hint\"",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard HTML accessibility missing %q", want)
		}
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	t.Parallel()

	rec := serveTestRequest(t, "/api/health")

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

	rec := serveTestRequest(t, "/api/report")

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

	rec := serveTestRequest(t, "/nonexistent")

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

func sseConnect(t *testing.T, url string) (*bufio.Scanner, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed via returned cleanup
	if err != nil {
		t.Fatalf("connect SSE: %v", err)
	}

	cleanup := func() {
		cancel()

		_ = resp.Body.Close()
	}

	return bufio.NewScanner(resp.Body), cleanup
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

	scanner, closeSSE := sseConnect(t, ts.URL+"/api/events")
	defer closeSSE()

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

	scanner, closeSSE := sseConnect(t, ts.URL+"/api/events")
	defer closeSSE()

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

	scanner, closeSSE := sseConnect(t, ts.URL+"/api/events")
	defer closeSSE()

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

	scanner1, closeSSE1 := sseConnect(t, ts.URL+"/api/events")
	defer closeSSE1()

	scanner2, closeSSE2 := sseConnect(t, ts.URL+"/api/events")
	defer closeSSE2()

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

	hub := live.NewHub()

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

	hub := live.NewHub()

	sub := hub.Subscribe()
	defer hub.Unsubscribe(sub.ID())

	evt := auditlog.Event{
		Sequence: 42,
		StepRef:  auditlog.StepRef{Name: "test"},
	}

	hub.OnEvent(evt)

	select {
	case received := <-sub.Events():
		var parsed struct {
			Sequence int `json:"sequence"`
		}

		err := json.Unmarshal(received.Data, &parsed)
		if err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}

		if parsed.Sequence != 42 {
			t.Errorf("expected sequence 42, got %d", parsed.Sequence)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestHub_SignalComplete(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

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

	hub := live.NewHub()

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

func TestHub_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	const goroutines = 20

	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	// Subscribers
	for range goroutines {
		go func() {
			defer wg.Done()

			for range iterations {
				sub := hub.Subscribe()
				_ = sub.ID()
				hub.Unsubscribe(sub.ID())
			}
		}()
	}

	// Event broadcasters
	for range goroutines {
		go func() {
			defer wg.Done()

			for i := range iterations {
				hub.OnEvent(auditlog.Event{
					Sequence:  i,
					StepRef:   auditlog.StepRef{Name: "concurrent-step"},
					EventType: auditlog.EventTypeAttemptStart,
					Phase:     auditlog.PhaseBefore,
				})
			}
		}()
	}

	// SignalComplete + IsComplete pollers
	for range goroutines {
		go func() {
			defer wg.Done()

			for range iterations {
				_ = hub.IsComplete()
				_ = hub.ClientCount()
			}
		}()
	}

	wg.Wait()

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after all goroutines finish, got %d", hub.ClientCount())
	}
}

func TestHub_UnsubscribeUnknownID(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	sub := hub.Subscribe()
	defer hub.Unsubscribe(sub.ID())

	// Unsubscribing a non-existent ID should be a no-op, not a panic.
	hub.Unsubscribe(99999)

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client after unknown unsubscribe, got %d", hub.ClientCount())
	}
}

func TestHub_OnEventMarshalError(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	sub := hub.Subscribe()
	defer hub.Unsubscribe(sub.ID())

	// OnEvent with a valid event should not panic even if marshaling
	// produces an unexpected result. A normal event should deliver fine.
	hub.OnEvent(auditlog.Event{
		Sequence: 1,
		StepRef:  auditlog.StepRef{Name: "test"},
	})

	select {
	case <-sub.Events():
	case <-time.After(time.Second):
		t.Fatal("did not receive event")
	}
}

// --- SSE Heartbeat Test ---

func TestServer_SSE_Heartbeat(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{
		Enabled: true,
		OnEvent: hub.OnEvent,
	})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{
		HeartbeatInterval: 50 * time.Millisecond,
	})

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect SSE: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)

	// Skip the initial snapshot
	skipSnapshot(scanner)

	deadline := time.After(2 * time.Second)

	for {
		select {
		case <-deadline:
			t.Fatal("did not receive heartbeat within timeout")
		default:
		}

		if !scanner.Scan() {
			t.Fatal("scanner ended before heartbeat")
		}

		line := scanner.Text()

		if strings.HasPrefix(line, ": heartbeat") {
			return
		}
	}
}

// --- Server Lifecycle Tests ---

func TestServer_ListenAndServe_Addr_Shutdown(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{
		Enabled: true,
		OnEvent: hub.OnEvent,
	})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{Addr: ":0"})

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	// Wait for server to start by polling health endpoint
	var lastErr error

	for range 50 {
		addr := server.Addr()

		if addr == "" || addr == ":0" {
			time.Sleep(20 * time.Millisecond)

			continue
		}

		url := "http://" + addr + "/api/health"

		ctx := t.Context()

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				lastErr = nil

				break
			}
		}

		lastErr = err

		time.Sleep(20 * time.Millisecond)
	}

	if lastErr != nil {
		t.Fatalf("server did not become ready: %v", lastErr)
	}

	// Verify Addr returns the real listen address
	addr := server.Addr()
	if addr == "" || addr == ":0" {
		t.Errorf("expected real listen address, got %q", addr)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestServer_SSE_ClientDisconnect(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithCancel(t.Context())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect SSE: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Give server time to register the subscriber
	time.Sleep(50 * time.Millisecond)

	if server.ClientCount() != 1 {
		t.Errorf("expected 1 client during active SSE connection, got %d", server.ClientCount())
	}

	// Disconnect client
	cancel()

	// Give server time to clean up
	time.Sleep(100 * time.Millisecond)

	if server.ClientCount() != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", server.ClientCount())
	}
}

func TestServer_NewInvalidConfig(t *testing.T) {
	t.Parallel()

	_, _, err := live.New(auditlog.Config{
		WorkflowID: "bad/name",
	}, live.Config{})
	if err == nil {
		t.Fatal("expected error for invalid WorkflowID")
	}
}

func TestServer_ShutdownNotRunning(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ctx := t.Context()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown on non-running server should return nil, got %v", err)
	}
}

// --- CORS Tests ---

func TestServer_CORSHeaders(t *testing.T) {
	t.Parallel()

	server := newTestServerWithCORS(t, "*")

	ctx := t.Context()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin '*', got %q", origin)
	}
}

func TestServer_CORSOptionsPreflight(t *testing.T) {
	t.Parallel()

	server := newTestServerWithCORS(t, "*")

	ctx := t.Context()

	req := httptest.NewRequestWithContext(ctx, http.MethodOptions, "/api/report", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS preflight, got %d", rec.Code)
	}
}

func TestServer_CORSDisabledByDefault(t *testing.T) {
	t.Parallel()

	server := newTestServer(t) // no CORS configured

	ctx := t.Context()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("expected no CORS header by default, got %q", origin)
	}
}

func TestServer_CORSSpecificOrigin(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{Enabled: true})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{
		CORSAllowedOrigins: "https://dashboard.example.com",
	})

	ctx := t.Context()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://dashboard.example.com" {
		t.Errorf("expected specific origin, got %q", origin)
	}
}

// --- Prefix Tests ---

func TestServer_PrefixRoutes(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{Enabled: true})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{
		Prefix: "/workflow",
	})

	ctx := t.Context()

	// Dashboard under prefix
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/workflow", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 at /workflow, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "workflow-auditlog") {
		t.Error("dashboard HTML should contain workflow-auditlog")
	}

	// API under prefix
	req2 := httptest.NewRequestWithContext(ctx, http.MethodGet, "/workflow/api/health", nil)
	rec2 := httptest.NewRecorder()

	server.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 at /workflow/api/health, got %d", rec2.Code)
	}
}

func TestServer_PrefixTrailingSlashStripped(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{Enabled: true})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{
		Prefix: "/dashboard/",
	})

	ctx := t.Context()

	// Should work at /dashboard/ (trailing slash stripped during normalization)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 at /dashboard, got %d", rec.Code)
	}
}

// --- Export Endpoint Tests ---

func TestServer_ExportNDJSON(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ctx := t.Context()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/export/ndjson", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "ndjson") {
		t.Errorf("expected ndjson content-type, got %s", ct)
	}

	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".ndjson") {
		t.Errorf("expected attachment disposition with .ndjson, got %s", cd)
	}
}

func TestServer_ExportHTML(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ctx := t.Context()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/export/html", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %s", ct)
	}

	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".html") {
		t.Errorf("expected attachment disposition with .html, got %s", cd)
	}

	if !strings.Contains(rec.Body.String(), "<!DOCTYPE html>") {
		t.Error("export HTML should contain DOCTYPE")
	}
}
