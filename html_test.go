package auditlog_test

import (
	"bytes"
	"fmt"
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

func TestWriteHTML_FromReplay(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSucceed("replay-html-a")
	s2 := newSucceed("replay-html-b")

	addDependentStep(w, s1, s2)
	runWorkflow(t, a, w)

	events := a.Events()

	report, err := auditlog.ReplayEvents(events)
	if err != nil {
		t.Fatalf("ReplayEvents: %v", err)
	}

	var buf strings.Builder

	err = report.WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML from replay: %v", err)
	}

	output := buf.String()
	assertContains(t, output, "<!DOCTYPE html>", "expected DOCTYPE from replayed report")
	assertContains(t, output, "replay-html-a", "expected step name from replayed report")
	assertContains(t, output, "replay-html-b", "expected step name from replayed report")
	assertContains(t, output, `"reconstructed":true`, "expected reconstructed flag in JSON data")
}

func TestWriteHTML_FromLoadedReport(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("loaded-html-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	var jsonBuf bytes.Buffer

	err := a.Report().WriteJSON(&jsonBuf)
	if err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	loaded, err := auditlog.LoadReportFromBytes(jsonBuf.Bytes())
	if err != nil {
		t.Fatalf("LoadReportFromBytes: %v", err)
	}

	var buf strings.Builder

	err = loaded.WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML from loaded: %v", err)
	}

	output := buf.String()
	assertContains(t, output, "<!DOCTYPE html>", "expected DOCTYPE from loaded report")
	assertContains(t, output, "loaded-html-step", "expected step name from loaded report")
}

func TestWriteHTML_DiamondDAG(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	root := newSucceed("diamond-root")
	left := newSucceed("diamond-left")
	right := newSucceed("diamond-right")
	sink := newSucceed("diamond-sink")

	w.Add(
		flow.Step(root),
		flow.Step(left).DependsOn(root),
		flow.Step(right).DependsOn(root),
		flow.Step(sink).DependsOn(left, right),
	)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	output := buf.String()
	assertContains(t, output, "diamond-root", "expected root step in HTML")
	assertContains(t, output, "diamond-left", "expected left branch step in HTML")
	assertContains(t, output, "diamond-right", "expected right branch step in HTML")
	assertContains(t, output, "diamond-sink", "expected sink step in HTML")
}

func TestWriteHTML_HighFanOut(t *testing.T) {
	t.Parallel()

	dur := 2.0
	steps := make([]auditlog.StepInfo, 11)
	steps[0] = auditlog.StepInfo{
		StepRef:    auditlog.StepRef{Name: "fan-root", StepType: "RootStep"},
		Status:     auditlog.StepStatusSucceeded,
		DurationMs: &dur,
	}

	for i := 1; i <= 10; i++ {
		steps[i] = auditlog.StepInfo{
			StepRef:      auditlog.StepRef{Name: fmt.Sprintf("fan-%d", i-1), StepType: "LeafStep"},
			Status:       auditlog.StepStatusSucceeded,
			DurationMs:   &dur,
			Dependencies: []auditlog.StepRef{{Name: "fan-root"}},
		}
	}

	report := auditlog.WorkflowReport{
		Version:        auditlog.SchemaVersion,
		WorkflowID:     "fanout-test",
		StepCount:      11,
		SucceededCount: 11,
		Steps:          steps,
	}

	var buf strings.Builder

	err := report.WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	output := buf.String()
	assertContains(t, output, "fan-root", "expected root step in HTML")
	assertContains(t, output, "fan-0", "expected first fan-out step in HTML")
	assertContains(t, output, "fan-9", "expected last fan-out step in HTML")
}

func TestWriteHTML_Determinism(t *testing.T) {
	t.Parallel()

	report := goldenHTMLReport()

	var buf1 strings.Builder

	err := report.WriteHTML(&buf1)
	if err != nil {
		t.Fatalf("first WriteHTML: %v", err)
	}

	var buf2 strings.Builder

	err = report.WriteHTML(&buf2)
	if err != nil {
		t.Fatalf("second WriteHTML: %v", err)
	}

	if buf1.String() != buf2.String() {
		t.Error("WriteHTML is not deterministic: same report produced different HTML on two calls")
	}
}

func TestWriteHTML_StructuralIntegrity(t *testing.T) {
	t.Parallel()

	output := writeSingleStepHTML(t, newSucceed("structure-test"))
	assertHTMLStructure(t, output)
}

// assertHTMLStructure validates the structural integrity of rendered HTML
// output: proper document type, balanced script tags, required elements, and
// a Content-Security-Policy meta tag. Used by HTML tests and the fuzz target.
func assertHTMLStructure(t *testing.T, output string) {
	t.Helper()

	if !strings.HasPrefix(output, "<!DOCTYPE html>") {
		t.Error("expected output to start with <!DOCTYPE html>")
	}

	for _, tag := range []string{"<html", "<head>", "<body>", "</html>", "Content-Security-Policy"} {
		if !strings.Contains(output, tag) {
			t.Errorf("expected %q in HTML output", tag)
		}
	}

	openCount := strings.Count(output, "<script")
	closeCount := strings.Count(output, "</script>")

	if openCount != 5 {
		t.Errorf("expected exactly 5 <script> tags, got %d", openCount)
	}

	if closeCount != 5 {
		t.Errorf("expected exactly 5 </script> tags, got %d", closeCount)
	}
}

func TestWriteHTML_FailureBanner_WhenFailed(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ok := newSucceed("ok-step")
	bad := newFail("bad-step", "explosion")
	addDependentStep(w, ok, bad)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "failure-banner", "expected failure-banner container in template")
	assertContains(t, output, `"workflow_succeeded":false`, "expected workflow_succeeded=false in JSON")
	assertContains(t, output, `"failure_reason"`, "expected failure_reason field in JSON")
	assertContains(t, output, `"bad-step"`, "expected failed step name in JSON")
	assertContains(t, output, "explosion", "expected error message in JSON data")
}

func TestWriteHTML_FailureBanner_HiddenWhenSucceeded(t *testing.T) {
	t.Parallel()

	output := writeSingleStepHTML(t, newSucceed("happy-step"))

	assertContains(t, output, `"workflow_succeeded":true`, "expected workflow_succeeded=true in JSON")
	assertContains(t, output, "failure-banner", "failure-banner template element should still exist (hidden by JS)")
}

func TestWriteHTML_ErrorColumn(t *testing.T) {
	t.Parallel()

	output := writeSingleStepHTML(t, newFail("crash-step", "disk full"))

	assertContains(t, output, ">Error</th>", "expected Error column header in template")
	assertContains(t, output, `colspan="9"`, "expected colspan=9 for empty state row")
	assertContains(t, output, `"error":"disk full"`, "expected error text in JSON data")
	assertContains(t, output, "error-cell", "expected error-cell CSS class reference")
}

func TestWriteHTML_WorkflowStatusBadge(t *testing.T) {
	t.Parallel()

	output := writeSingleStepHTML(t, newSucceed("pass-step"))

	assertContains(t, output, "workflow-status", "expected workflow-status badge element in template")
	assertContains(t, output, `"workflow_succeeded":true`, "expected success status in JSON")
	assertContains(t, output, "workflow-status passed", "expected passed CSS class in JS")
	assertContains(t, output, "workflow-status failed", "expected failed CSS class in JS")
}

func TestWriteHTML_GanttChart(t *testing.T) {
	t.Parallel()

	output := writeSingleStepHTML(t, newSucceed("timed-step"))

	assertContains(t, output, "gantt-axis", "expected gantt-axis CSS in template")
	assertContains(t, output, "gantt-grid", "expected gantt-grid CSS in template")
	assertContains(t, output, "gantt-bar", "expected gantt-bar CSS in template")
	assertContains(t, output, "renderGantt", "expected Gantt render function in JS")
	assertContains(t, output, `"started_at"`, "expected started_at in JSON")
	assertContains(t, output, `"finished_at"`, "expected finished_at in JSON")
}

func TestWriteHTML_ImpactBadge(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	root := newFail("root-fail", "root broken")
	child := newSucceed("child-step")
	addDependentStep(w, root, child)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteHTML(&buf)
	if err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "impact-badge", "expected impact-badge CSS class")
	assertContains(t, output, "impactedSteps", "expected impactedSteps computation in JS")
	assertContains(t, output, "computeImpact", "expected computeImpact function in JS")
}

func TestWriteHTML_HumanizedDurations(t *testing.T) {
	t.Parallel()

	output := writeSingleStepHTML(t, newSucceed("duration-step"))

	assertContains(t, output, "function humanizeDuration", "expected humanizeDuration function definition")
	assertContains(
		t,
		output,
		"humanizeDuration(report.wall_clock_duration_ms)",
		"expected humanized wall clock in stats",
	)
	assertContains(t, output, "humanizeDuration(s.duration_ms)", "expected humanized duration in steps table")
}

func TestWriteHTML_GraphFailedNodeDot(t *testing.T) {
	t.Parallel()

	output := writeSingleStepHTML(t, newFail("graph-fail", "node error"))

	// Error dot rendering is now in the daghtml SDK JS, triggered by the
	// "error":true field in the DAG JSON data.
	assertContains(t, output, `"error":true`, "expected error:true in DAG JSON for failed step")
}

func TestWriteHTML_TreeInlineError(t *testing.T) {
	t.Parallel()

	output := writeSingleStepHTML(t, newFail("tree-fail", "tree error here"))

	assertContains(t, output, "scope-node-error", "expected scope-node-error CSS class for tree inline errors")
	assertContains(t, output, "has-failure", "expected has-failure class for failed tree nodes")
	assertContains(t, output, `"tree error here"`, "expected error text in JSON data")
}
