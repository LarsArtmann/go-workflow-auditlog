package live

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

var errProviderFailure = errors.New("provider failure")

func TestServer_NilReportProvider(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		mux: http.NewServeMux(),
	}

	ctx := t.Context()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/report", nil)
	rec := httptest.NewRecorder()

	srv.handleReport(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for nil reportProvider, got %d", rec.Code)
	}
}

func TestServer_SendSnapshotNilProvider(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		mux: http.NewServeMux(),
	}

	rec := httptest.NewRecorder()

	err := srv.sendSnapshot(rec, rec)
	if err != nil {
		t.Errorf("expected nil error for nil snapshotProvider, got %v", err)
	}
}

func TestServer_SendCompleteNilProvider(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		mux: http.NewServeMux(),
	}

	rec := httptest.NewRecorder()

	srv.sendComplete(rec, rec)
}

func TestServer_NewErrorInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := auditlog.New(auditlog.Config{
		Enabled:    true,
		WorkflowID: "bad/name",
	})
	if err == nil {
		t.Fatal("expected error for path separator in WorkflowID")
	}
}

func TestServer_ListenAndServeAlreadyRunning(t *testing.T) {
	t.Parallel()

	auditor, err := auditlog.New(auditlog.Config{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(NewHub(), auditor, Config{Addr: ":0"})

	go func() {
		_ = srv.ListenAndServe()
	}()

	time.Sleep(50 * time.Millisecond)

	err = srv.ListenAndServe()
	if err == nil {
		t.Fatal("expected ErrServerAlreadyRunning on second ListenAndServe")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_ = srv.Shutdown(ctx)
}

func TestServer_NilNDJSONWriter(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		mux: http.NewServeMux(),
	}

	ctx := t.Context()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/export/ndjson", nil)
	rec := httptest.NewRecorder()

	srv.handleExportNDJSON(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for nil ndjsonWriter, got %d", rec.Code)
	}
}

func TestServer_NilHTMLWriter(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		mux: http.NewServeMux(),
	}

	ctx := t.Context()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/export/html", nil)
	rec := httptest.NewRecorder()

	srv.handleExportHTML(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for nil htmlWriter, got %d", rec.Code)
	}
}

func TestServer_SendWSCompleteNilProvider(t *testing.T) {
	t.Parallel()

	srv := &Server{hub: NewHub()}
	// Should not panic with nil completeProvider
	srv.sendWSComplete(nil)
}

func TestServer_HandleWebSocketNilSnapshot(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		mux: http.NewServeMux(),
	}

	// Without snapshotProvider, handleWebSocket should still work
	// (sends no snapshot, waits for events)
	srv.hub.SignalComplete()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/ws", nil)
	rec := httptest.NewRecorder()

	srv.handleWebSocket(rec, req)
}

func TestNormalizePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"", "/"},
		{"/", "/"},
		{"/prefix", "/prefix"},
		{"/prefix/", "/prefix"},
		{"prefix", "/prefix"},
		{"prefix/", "/prefix"},
	}

	for _, tc := range tests {
		got := normalizePrefix(tc.input)
		if got != tc.want {
			t.Errorf("normalizePrefix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestServer_ShutdownNotStarted(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		mux: http.NewServeMux(),
	}

	ctx := t.Context()

	err := srv.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown on unstarted server should return nil, got %v", err)
	}
}

func TestServer_HandleHealthWithProvider(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		mux: http.NewServeMux(),
		healthProvider: func() HealthInfo {
			return HealthInfo{Events: 42, Dropped: 3}
		},
	}

	ctx := t.Context()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	srv.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "42") {
		t.Errorf("health response should contain events count 42: %s", body)
	}
}

// --- Provider / write-failure coverage ---

// failingFlusher is an http.ResponseWriter + http.Flusher whose Write always
// fails. Used to exercise SSE write-error branches.
type failingFlusher struct {
	header http.Header
}

func (f *failingFlusher) Write([]byte) (int, error) { return 0, errProviderFailure }
func (f *failingFlusher) Header() http.Header {
	if f.header == nil {
		f.header = http.Header{}
	}

	return f.header
}

func (f *failingFlusher) WriteHeader(int) {}
func (f *failingFlusher) Flush()          {}

// failAfterFlushWriter succeeds for all Write calls until the first Flush,
// then fails every subsequent Write. This lets a snapshot fully flush before
// a heartbeat write fails — robust regardless of snapshot payload size (no
// magic write-count threshold).
type failAfterFlushWriter struct {
	flushed bool
	header  http.Header
}

func (f *failAfterFlushWriter) Write(p []byte) (int, error) {
	if f.flushed {
		return 0, errProviderFailure
	}

	return len(p), nil
}

func (f *failAfterFlushWriter) Header() http.Header {
	if f.header == nil {
		f.header = http.Header{}
	}

	return f.header
}

func (f *failAfterFlushWriter) WriteHeader(int) {}
func (f *failAfterFlushWriter) Flush()          { f.flushed = true }

// nonFlusherWriter wraps a ResponseRecorder but hides its Flush method so the
// SSE handler sees a writer that does not support streaming.
type nonFlusherWriter struct {
	http.ResponseWriter
}

func TestServer_HandleSSE_StreamingNotSupported(t *testing.T) {
	t.Parallel()

	srv := &Server{hub: NewHub()}

	ctx := t.Context()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()

	srv.handleSSE(nonFlusherWriter{rec}, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when writer is not a Flusher, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "streaming not supported") {
		t.Errorf("expected streaming-not-supported message, got: %s", rec.Body.String())
	}
}

func TestServer_HandleReportProviderError(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub:            NewHub(),
		mux:            http.NewServeMux(),
		reportProvider: func() ([]byte, error) { return nil, errProviderFailure },
	}

	ctx := t.Context()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/report", nil)
	rec := httptest.NewRecorder()

	srv.handleReport(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for failing reportProvider, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "generate report") {
		t.Errorf("expected generate-report error message, got: %s", rec.Body.String())
	}
}

func TestServer_HandleExportNDJSONWriterError(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub:          NewHub(),
		mux:          http.NewServeMux(),
		ndjsonWriter: func(io.Writer) error { return errProviderFailure },
	}

	ctx := t.Context()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/export/ndjson", nil)
	rec := httptest.NewRecorder()

	srv.handleExportNDJSON(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for failing ndjsonWriter, got %d", rec.Code)
	}
}

func TestServer_HandleExportHTMLWriterError(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub:        NewHub(),
		mux:        http.NewServeMux(),
		htmlWriter: func(io.Writer) error { return errProviderFailure },
	}

	ctx := t.Context()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/export/html", nil)
	rec := httptest.NewRecorder()

	srv.handleExportHTML(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for failing htmlWriter, got %d", rec.Code)
	}
}

func TestServer_SendSnapshotProviderError(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		snapshotProvider: func(bool) (jsontext.Value, error) {
			return nil, errProviderFailure
		},
	}

	rec := httptest.NewRecorder()

	err := srv.sendSnapshot(rec, rec)
	if err == nil {
		t.Fatal("expected error from failing snapshotProvider")
	}

	if !strings.Contains(err.Error(), "build snapshot") {
		t.Errorf("expected build-snapshot error wrap, got: %v", err)
	}
}

func TestServer_SendCompleteProviderError(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		completeProvider: func() (jsontext.Value, error) {
			return nil, errProviderFailure
		},
	}

	rec := httptest.NewRecorder()

	// Must not panic and must return without writing a complete event.
	srv.sendComplete(rec, rec)

	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body when completeProvider errors, got: %s", rec.Body.String())
	}
}

// TestServer_HandleSSE_SnapshotWriteFailure drives handleSSE with a writer
// whose Write always fails, so sendSnapshot's WriteEvent errors and the
// handler returns immediately.
func TestServer_HandleSSE_SnapshotWriteFailure(t *testing.T) {
	t.Parallel()

	auditor, err := auditlog.New(auditlog.Config{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(NewHub(), auditor, Config{})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)

	done := make(chan struct{})

	go func() {
		srv.handleSSE(&failingFlusher{}, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("handleSSE did not return after snapshot write failure")
	}
}

// TestServer_HandleSSE_HeartbeatWriteFailure lets the snapshot flush, then the
// next heartbeat write fails and the handler returns.
func TestServer_HandleSSE_HeartbeatWriteFailure(t *testing.T) {
	t.Parallel()

	auditor, err := auditlog.New(auditlog.Config{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(NewHub(), auditor, Config{HeartbeatInterval: time.Millisecond})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)

	done := make(chan struct{})

	go func() {
		// Allow the snapshot to flush, then fail on a subsequent heartbeat.
		srv.handleSSE(&failAfterFlushWriter{}, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("handleSSE did not return after heartbeat write failure")
	}
}

// TestServer_HandleSSE_EventWriteFailure disables the snapshot (nil provider)
// and heartbeat, injects an event, and verifies the handler returns when the
// event WriteEvent fails.
func TestServer_HandleSSE_EventWriteFailure(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub:    NewHub(),
		mux:    http.NewServeMux(),
		config: Config{HeartbeatInterval: time.Hour}, // disable heartbeat
		// snapshotProvider nil -> sendSnapshot returns nil immediately
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)

	done := make(chan struct{})

	go func() {
		srv.handleSSE(&failingFlusher{}, req)
		close(done)
	}()

	// Give the handler time to subscribe before broadcasting, so the event
	// lands in the subscriber's buffer.
	time.Sleep(20 * time.Millisecond)

	srv.hub.OnEvent(auditlog.Event{
		Sequence:  1,
		StepRef:   auditlog.StepRef{Name: "ev-write-fail"},
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("handleSSE did not return after event write failure")
	}
}

// TestServer_SendWSCompleteProviderError exercises the WebSocket complete path
// when the completeProvider errors: it must return without touching the
// connection. A nil conn is safe because the error branch returns before any
// write.
func TestServer_SendWSCompleteProviderError(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		completeProvider: func() (jsontext.Value, error) {
			return nil, errProviderFailure
		},
	}

	// Must not panic and must return without writing (conn is nil by design).
	srv.sendWSComplete(nil)
}

// TestServer_HandleWebSocket_SnapshotProviderError verifies graceful
// degradation over a real WebSocket upgrade: when snapshotProvider errors, the
// snapshot is skipped, and the FIRST frame the client receives is the
// completion (never a snapshot).
func TestServer_HandleWebSocket_SnapshotProviderError(t *testing.T) {
	t.Parallel()

	auditor, err := auditlog.New(auditlog.Config{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(NewHub(), auditor, Config{})
	srv.snapshotProvider = func(bool) (jsontext.Value, error) {
		return nil, errProviderFailure
	}

	ts := httptest.NewServer(srv)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/ws"

	conn, resp, dialErr := websocket.DefaultDialer.DialContext(t.Context(), wsURL, nil)
	if resp != nil {
		_ = resp.Body.Close()
	}

	if dialErr != nil {
		t.Fatalf("dial websocket: %v", dialErr)
	}

	defer func() { _ = conn.Close() }()

	// Let the handler skip the failed snapshot and reach its select loop.
	time.Sleep(30 * time.Millisecond)

	srv.SignalComplete()

	// The first frame must be "complete" — never "snapshot" — proving the
	// snapshot was skipped when its provider errored.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	_, msg, readErr := conn.ReadMessage()
	if readErr != nil {
		t.Fatalf("expected complete frame, got read error: %v", readErr)
	}

	if !strings.Contains(string(msg), `"complete"`) {
		t.Errorf("expected first frame to be complete (snapshot skipped), got: %s", string(msg))
	}
}
