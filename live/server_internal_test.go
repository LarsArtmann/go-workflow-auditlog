package live

import (
	"net/http"
	"net/http/httptest"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestServer_NilReportProvider(t *testing.T) {
	t.Parallel()

	srv := &Server{ //nolint:exhaustruct
		hub: NewHub(),
		mux: http.NewServeMux(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/report", nil)
	rec := httptest.NewRecorder()

	srv.handleReport(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for nil reportProvider, got %d", rec.Code)
	}
}

func TestServer_SendSnapshotNilProvider(t *testing.T) {
	t.Parallel()

	srv := &Server{ //nolint:exhaustruct
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

	srv := &Server{ //nolint:exhaustruct
		hub: NewHub(),
		mux: http.NewServeMux(),
	}

	rec := httptest.NewRecorder()

	srv.sendComplete(rec, rec)
}

func TestServer_NewErrorInvalidConfig(t *testing.T) {
	t.Parallel()

	_, _, err := auditlog.New(auditlog.Config{
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

	srv := NewServer(NewHub(), &auditor, Config{Addr: ":0"})

	if err := srv.ListenAndServe(); err == nil {
		t.Fatal("expected ListenAndServe to block/fail, got nil")
	}

	if err := srv.ListenAndServe(); err == nil {
		t.Fatal("expected ErrServerAlreadyRunning on second ListenAndServe")
	}
}
