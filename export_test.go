package auditlog_test

import (
	"errors"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

// TestReportWriteNDJSON_FlushError verifies that when the underlying writer
// fails during flush, WriteNDJSON wraps the error with ErrExportWriteFailed.
// This exercises the writeEventsNDJSON flush error path from the core module
// (previously only covered indirectly from the viz test suite).
func TestReportWriteNDJSON_FlushError(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "flush-error-step")
	report := a.Report()

	err := report.WriteNDJSON(testhelpers.FailingWriter{})
	if err == nil {
		t.Fatal("expected error from FailingWriter")
	}

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("expected ErrExportWriteFailed, got: %v", err)
	}
}

// TestReportWriteJSON_RenderError verifies that JSON encoding to a failing
// writer wraps the error with ErrRenderFailed.
func TestReportWriteJSON_RenderError(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "render-error-step")
	report := a.Report()

	err := report.WriteJSON(testhelpers.FailingWriter{})
	if err == nil {
		t.Fatal("expected error from FailingWriter")
	}

	if !errors.Is(err, auditlog.ErrRenderFailed) {
		t.Errorf("expected ErrRenderFailed, got: %v", err)
	}
}
