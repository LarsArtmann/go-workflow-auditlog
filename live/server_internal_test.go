package live

import (
	"context"
	"net/http"
	"net/http/httptest"
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
