package auditlog_test

import (
	"bytes"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

func TestLoadReport_RoundTrip(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s1 := testhelpers.NewSucceed("load-step-1")
	s2 := testhelpers.NewFail("load-step-2", "err")
	testhelpers.AddParallelSteps(w, s1, s2)
	testhelpers.RunWorkflow(t, a, w)

	// Export to JSON.
	var buf bytes.Buffer

	err := a.WriteJSON(&buf)
	if err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Load it back.
	loaded, err := auditlog.LoadReportFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadReportFromBytes: %v", err)
	}

	if loaded.WorkflowID != "test" {
		t.Errorf("expected WorkflowID 'test', got %q", loaded.WorkflowID)
	}

	testhelpers.AssertStepCount(t, loaded, 2)

	if loaded.FailedCount != 1 {
		t.Errorf("expected 1 failed, got %d", loaded.FailedCount)
	}
}

func TestLoadReport_FromFile(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "file-load-step", "report.json")

	err := a.ExportJSON(path)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	loaded, err := auditlog.LoadReport(path)
	if err != nil {
		t.Fatalf("LoadReport: %v", err)
	}

	testhelpers.AssertStepCount(t, loaded, 1)
}

func TestLoadReport_FromReader(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "reader-step")

	var buf bytes.Buffer

	_ = a.WriteJSON(&buf)

	loaded, err := auditlog.LoadReportFromReader(&buf)
	if err != nil {
		t.Fatalf("LoadReportFromReader: %v", err)
	}

	testhelpers.AssertStepCount(t, loaded, 1)
}

func TestLoadReport_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := auditlog.LoadReportFromBytes([]byte(`{"bad json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadReport_NonexistentFile(t *testing.T) {
	t.Parallel()

	_, err := auditlog.LoadReport("/nonexistent/path/report.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestReportWriteNDJSON(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "ndjson-report-step")

	report := a.Report()

	var buf bytes.Buffer

	err := report.WriteNDJSON(&buf)
	if err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected non-empty NDJSON output")
	}

	// Verify we can read it back.
	events, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != report.EventCount {
		t.Errorf("expected %d events, got %d", report.EventCount, len(events))
	}
}
