package auditlog_test

import (
	"os"
	"strings"
	"testing"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestWriteHTML_BasicReport(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	transform := newSucceed("transform")
	save := newSucceed("save")
	addLinearChain(w, fetch, transform, save)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "<!DOCTYPE html>", "expected DOCTYPE")
	assertContains(t, output, "<html", "expected html element")
	assertContains(t, output, "</html>", "expected closing html tag")
	assertContains(t, output, "workflow-auditlog", "expected library name in header")
	assertContains(t, output, `id="report-data"`, "expected report-data script tag")
	assertContains(t, output, `id="type-metadata"`, "expected type-metadata script tag")
	assertContains(t, output, `"step_name":"fetch"`, "expected fetch step in JSON data")
	assertContains(t, output, `"step_name":"transform"`, "expected transform step in JSON data")
	assertContains(t, output, `"step_name":"save"`, "expected save step in JSON data")
	assertContains(t, output, "Content-Security-Policy", "expected CSP header")
	assertContains(t, output, "attempt_start", "expected event type in data")
	assertContains(t, output, "attempt_end", "expected event type in data")
}

func TestWriteHTML_EmptyReport(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		Version:    auditlog.SchemaVersion,
		WorkflowID: "empty",
		StepCount:  0,
		EventCount: 0,
		Steps:      []auditlog.StepInfo{},
		Events:     []auditlog.Event{},
	}

	var buf strings.Builder

	err := report.WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML error: %v", err)
	}

	output := buf.String()
	assertContains(t, output, "<!DOCTYPE html>", "expected DOCTYPE even for empty report")
	assertContains(t, output, `"workflow_id":"empty"`, "expected workflow_id in JSON")
}

func TestWriteHTML_FailedStepWithError(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ok := newSucceed("ok-step")
	bad := newFail("bad-step", "explosion")
	addDependentStep(w, ok, bad)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML error: %v", err)
	}

	output := buf.String()
	assertContains(t, output, "bad-step", "expected failed step name")
	assertContains(t, output, "explosion", "expected error message in JSON data")
	assertContains(t, output, `"failed"`, "expected failed status")
}

func TestWriteHTMLString_ReturnsContent(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("only-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	output, err := a.Report().WriteHTMLString()
	if err != nil {
		t.Fatalf("WriteHTMLString error: %v", err)
	}

	if len(output) == 0 {
		t.Fatal("WriteHTMLString returned empty string")
	}

	if !strings.HasPrefix(output, "<!DOCTYPE html>") {
		t.Error("WriteHTMLString output should start with DOCTYPE")
	}
}

func TestExportHTML_WritesFile(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("exported-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	path := t.TempDir() + "/report.html"

	err := a.Report().ExportHTML(path)
	if err != nil {
		t.Fatalf("ExportHTML error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("exported HTML file is empty")
	}

	if !strings.Contains(string(data), "exported-step") {
		t.Error("exported HTML should contain step name")
	}
}

func TestAuditor_WriteHTML_DelegatesToReport(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("delegate-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.WriteHTML(&buf)
	if err != nil {
		t.Fatalf("Auditor.WriteHTML error: %v", err)
	}

	assertContains(t, buf.String(), "delegate-step", "expected step name in Auditor.WriteHTML output")
}

func TestAuditor_ExportHTML_DelegatesToReport(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("auditor-export")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	path := t.TempDir() + "/auditor-report.html"

	err := a.ExportHTML(path)
	if err != nil {
		t.Fatalf("Auditor.ExportHTML error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if !strings.Contains(string(data), "auditor-export") {
		t.Error("expected step name in Auditor.ExportHTML output")
	}
}

func TestAuditor_WriteHTMLString_DelegatesToReport(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("string-delegate")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	output, err := a.WriteHTMLString()
	if err != nil {
		t.Fatalf("Auditor.WriteHTMLString error: %v", err)
	}

	assertContains(t, output, "string-delegate", "expected step name in string output")
}

func TestWriteHTML_RetryStepHasAttemptCount(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	flaky := newFlaky("flaky-retry", 2)
	addRetryStep(w, flaky, 5)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML error: %v", err)
	}

	output := buf.String()
	assertContains(t, output, "flaky-retry", "expected retry step name")
	assertContains(t, output, `"attempt_count":3`, "expected 3 attempts (2 fail + 1 success)")
	assertContains(t, output, `"has_retry":true`, "expected has_retry flag")
}

func TestWriteHTML_MetadataInjected(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		Version:    auditlog.SchemaVersion,
		WorkflowID: "meta-test",
		Steps:      []auditlog.StepInfo{},
		Events:     []auditlog.Event{},
	}

	var buf strings.Builder

	err := report.WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML error: %v", err)
	}

	output := buf.String()
	assertContains(t, output, `"statuses"`, "expected statuses in type metadata")
	assertContains(t, output, `"events"`, "expected events in type metadata")
	assertContains(t, output, `"succeeded"`, "expected succeeded status in metadata")
	assertContains(t, output, `"attempt_start"`, "expected attempt_start event in metadata")
}

func TestWriteHTML_AllSixStatuses(t *testing.T) {
	t.Parallel()

	dur := 1.5
	errMsg := "step error"

	report := auditlog.WorkflowReport{
		Version:             auditlog.SchemaVersion,
		WorkflowID:          "status-test",
		StepCount:           6,
		SucceededCount:      1,
		FailedCount:         1,
		SkippedCount:        1,
		CanceledCount:       1,
		PendingCount:        1,
		RunningCount:        1,
		EventCount:          0,
		WallClockDurationMs: 10.0,
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "s1"}, Status: auditlog.StepStatusSucceeded, DurationMs: &dur},
			{
				StepRef:    auditlog.StepRef{Name: "s2"},
				Status:     auditlog.StepStatusFailed,
				DurationMs: &dur,
				Error:      &errMsg,
			},
			{StepRef: auditlog.StepRef{Name: "s3"}, Status: auditlog.StepStatusSkipped},
			{StepRef: auditlog.StepRef{Name: "s4"}, Status: auditlog.StepStatusCanceled, Error: &errMsg},
			{StepRef: auditlog.StepRef{Name: "s5"}, Status: auditlog.StepStatusPending},
			{StepRef: auditlog.StepRef{Name: "s6"}, Status: auditlog.StepStatusRunning},
		},
	}

	var buf strings.Builder

	err := report.WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML error: %v", err)
	}

	output := buf.String()
	for _, status := range []string{"succeeded", "failed", "skipped", "canceled", "pending", "running"} {
		assertContains(t, output, status, "expected status '"+status+"' in HTML output")
	}
}
