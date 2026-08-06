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

	"github.com/larsartmann/go-sse"
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/events", nil)
	stream := sse.NewStream(rec, req)

	err := srv.sendSnapshot(stream)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/events", nil)
	stream := sse.NewStream(rec, req)

	srv.sendComplete(stream)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/events", nil)
	stream := sse.NewStream(rec, req)

	err := srv.sendSnapshot(stream)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/events", nil)
	stream := sse.NewStream(rec, req)

	// Must not panic and must return without writing a complete event.
	srv.sendComplete(stream)

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

// TestServer_HandleSSE_SnapshotProviderError drives handleSSE end-to-end with a
// snapshotProvider that always errors, verifying the handler returns
// gracefully (no panic, no hang) instead of entering the event loop. This is
// the SSE equivalent of the removed WebSocket provider-error test.
func TestServer_HandleSSE_SnapshotProviderError(t *testing.T) {
	t.Parallel()

	srv := &Server{
		hub: NewHub(),
		snapshotProvider: func(bool) (jsontext.Value, error) {
			return nil, errProviderFailure
		},
	}

	rec := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)

	done := make(chan struct{})

	go func() {
		srv.handleSSE(rec, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("handleSSE did not return after snapshotProvider error")
	}
}

// TestServer_HandleSSE_WriteFailureAfterSnapshot lets the snapshot flush,
// then verifies the handler exits when a subsequent event Send fails.
// With Stream.Heartbeat running in a goroutine, heartbeat write failures are
// handled internally by go-sse (the goroutine exits silently); this test now
// covers the event Send failure path after a successful snapshot flush.
func TestServer_HandleSSE_WriteFailureAfterSnapshot(t *testing.T) {
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
		// Allow the snapshot to flush, then fail on a subsequent event Send.
		srv.handleSSE(&failAfterFlushWriter{}, req)
		close(done)
	}()

	// Wait for the handler to subscribe and flush the snapshot, then
	// broadcast an event whose Send will fail on the dead writer.
	time.Sleep(30 * time.Millisecond)

	srv.hub.OnEvent(auditlog.Event{
		Sequence:  1,
		StepRef:   auditlog.StepRef{Name: "post-snapshot-fail"},
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("handleSSE did not return after event write failure")
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
