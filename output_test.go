package auditlog_test

import (
	"strings"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// --- D2 Diagram Tests ---

// renderMarkdownTable renders the report's step summary as Markdown into a
// buffer and returns it. Centralizes the
// `var buf strings.Builder; err := a.Report().WriteTable(&buf, output.FormatMarkdown, output.RenderOptions{})`
// boilerplate used by output tests that need a Markdown rendering of a report.
func renderMarkdownTable(t *testing.T, a *auditlog.Auditor) string {
	t.Helper()

	var buf strings.Builder

	err := a.Report().WriteTable(&buf, output.FormatMarkdown, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTable markdown error: %v", err)
	}

	return buf.String()
}

// --- D2 Diagram Tests ---

func TestD2_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	transform := newSucceed("transform")
	save := newSucceed("save")

	addDependentStep(w, fetch, transform)
	addDependentStep(w, transform, save)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteD2(&buf)
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "title:", "expected title block in D2 output")
	assertContains(t, output, "fetch", "expected 'fetch' node in output")
	assertContains(t, output, "transform", "expected 'transform' node in output")
	assertContains(t, output, "save", "expected 'save' node in output")
	assertContains(t, output, "->", "expected '->' edge in output")
	assertContains(t, output, "#2d5a2d", "expected green fill color for succeeded step")
}

func TestD2_FailedStepRedColor(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	bad := newFail("bad", "boom")
	w.Add(flow.Step(bad))
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteD2(&buf)
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	output := buf.String()
	assertContains(t, output, "#8b2d2d", "expected red fill color for failed step")
}

func TestD2_EmptyReport(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})

	var buf strings.Builder

	err := a.Report().WriteD2(&buf)
	if err != nil {
		t.Fatalf("WriteD2 on empty report error: %v", err)
	}

	assertContains(t, buf.String(), "title:", "expected title block even for empty report")
}

func TestWriteD2String(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("d2-string")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	output, err := a.Report().WriteD2String()
	if err != nil {
		t.Fatalf("WriteD2String error: %v", err)
	}

	assertContains(t, output, "title:", "expected 'title:' in string output")
}

func TestExportD2(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("export-d2")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	path := t.TempDir() + "/dag.d2"

	err := a.ExportD2(path)
	if err != nil {
		t.Fatalf("ExportD2 error: %v", err)
	}
}

// --- Table Tests ---

func TestWriteTable_Markdown(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	bad := newFail("bad", "boom")
	w.Add(flow.Step(fetch), flow.Step(bad))
	runWorkflow(t, a, w)

	output := renderMarkdownTable(t, a)

	assertContains(t, output, "Step", "expected markdown table header")
	assertContains(t, output, "fetch", "expected fetch row")
	assertContains(t, output, "bad", "expected bad row")
	assertContains(t, output, "succeeded", "expected succeeded status")
	assertContains(t, output, "failed", "expected failed status")
}

func TestWriteTable_CSV(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("csv-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteTable(&buf, output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTable csv error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "Step,Status,Duration,Attempts,Error", "expected CSV header")
	assertContains(t, output, "csv-step", "expected step name in CSV")
	assertContains(t, output, "succeeded", "expected status in CSV")
}

func TestWriteTable_JSON(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("json-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteTable(&buf, output.FormatJSON, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTable json error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "json-step", "expected step name in JSON")
	assertContains(t, output, "succeeded", "expected status in JSON")
}

func TestWriteTable_JSONL(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("jsonl-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteTable(&buf, output.FormatJSONL, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTable jsonl error: %v", err)
	}

	output := buf.String()

	// JSONL: one object per line.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 JSONL line, got %d", len(lines))
	}

	assertContains(t, output, "jsonl-step", "expected step name in JSONL")
}

func TestWriteTable_EmptyReport(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})

	output := renderMarkdownTable(t, a)
	assertContains(t, output, "| Step |", "expected header even for empty report")
}

func TestWriteTableString(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("table-string")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	output, err := a.Report().WriteTableString(output.FormatMarkdown, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	assertContains(t, output, "table-string", "expected step name in table string")
}

func TestExportTable(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("export-table")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	path := t.TempDir() + "/report.csv"

	err := a.ExportTable(path, output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("ExportTable error: %v", err)
	}
}

// --- Tree Tests ---

func TestWriteTree_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	transform := newSucceed("transform")
	addDependentStep(w, fetch, transform)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteTree(&buf)
	if err != nil {
		t.Fatalf("WriteTree error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "Workflow", "expected root label")
	assertContains(t, output, "fetch", "expected fetch in tree")
	assertContains(t, output, "transform", "expected transform in tree")
}

func TestWriteTree_EmptyReport(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})

	var buf strings.Builder

	err := a.Report().WriteTree(&buf)
	if err != nil {
		t.Fatalf("WriteTree on empty report error: %v", err)
	}

	assertContains(t, buf.String(), "Workflow", "expected root label even for empty report")
}

func TestWriteTreeString(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("tree-string")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	output, err := a.Report().WriteTreeString()
	if err != nil {
		t.Fatalf("WriteTreeString error: %v", err)
	}

	assertContains(t, output, "tree-string", "expected step name in tree string")
}

func TestWriteHTMLTree_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	transform := newSucceed("transform")
	addDependentStep(w, fetch, transform)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteHTMLTree(&buf)
	if err != nil {
		t.Fatalf("WriteHTMLTree error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "<ul>", "expected <ul> in HTML tree")
	assertContains(t, output, "fetch", "expected fetch in HTML tree")
	assertContains(t, output, "transform", "expected transform in HTML tree")
}

func TestWriteHTMLTreeString(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("html-tree-string")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	output, err := a.Report().WriteHTMLTreeString()
	if err != nil {
		t.Fatalf("WriteHTMLTreeString error: %v", err)
	}

	assertContains(t, output, "<ul>", "expected <ul> in HTML tree string")
}

func TestExportTree(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("export-tree")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	path := t.TempDir() + "/tree.txt"

	err := a.ExportTree(path)
	if err != nil {
		t.Fatalf("ExportTree error: %v", err)
	}
}

func TestExportHTMLTree(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("export-html-tree")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	path := t.TempDir() + "/tree.html"

	err := a.ExportHTMLTree(path)
	if err != nil {
		t.Fatalf("ExportHTMLTree error: %v", err)
	}
}
