package auditlog_test

import (
	"bytes"
	"testing"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestLoadReport_RoundTrip(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSucceed("load-step-1")
	s2 := newFail("load-step-2", "err")
	addParallelSteps(w, s1, s2)
	runWorkflow(t, a, w)

	// Export to JSON.
	var buf bytes.Buffer

	err := a.WriteReportJSON(&buf)
	if err != nil {
		t.Fatalf("WriteReportJSON: %v", err)
	}

	// Load it back.
	loaded, err := auditlog.LoadReportFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadReportFromBytes: %v", err)
	}

	if loaded.WorkflowID != "test" {
		t.Errorf("expected WorkflowID 'test', got %q", loaded.WorkflowID)
	}

	assertStepCount(t, loaded, 2)

	if loaded.FailedCount != 1 {
		t.Errorf("expected 1 failed, got %d", loaded.FailedCount)
	}
}

func TestLoadReport_FromFile(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("file-load-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	path := t.TempDir() + "/report.json"

	err := a.ExportToFile(path)
	if err != nil {
		t.Fatalf("ExportToFile: %v", err)
	}

	loaded, err := auditlog.LoadReport(path)
	if err != nil {
		t.Fatalf("LoadReport: %v", err)
	}

	assertStepCount(t, loaded, 1)
}

func TestLoadReport_FromReader(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("reader-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	var buf bytes.Buffer

	_ = a.WriteReportJSON(&buf)

	loaded, err := auditlog.LoadReportFromReader(&buf)
	if err != nil {
		t.Fatalf("LoadReportFromReader: %v", err)
	}

	assertStepCount(t, loaded, 1)
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

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("ndjson-report-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

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
