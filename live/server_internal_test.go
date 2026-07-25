package live

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

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
