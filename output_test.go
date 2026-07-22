package auditlog_test

import (
	"os"
	"path/filepath"
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
	assertContains(t, output, "test", "expected D2 title to derive from WorkflowID 'test'")
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

	a := runSingleSucceed(t, "d2-string")

	output, err := a.Report().WriteD2String()
	if err != nil {
		t.Fatalf("WriteD2String error: %v", err)
	}

	assertContains(t, output, "title:", "expected 'title:' in string output")
}

func TestExportD2(t *testing.T) {
	t.Parallel()

	a, path := singleSucceedExportPath(t, "export-d2", "dag.d2")

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

	a, buf := runSingleSucceedWithBuffer(t, "csv-step")

	err := a.Report().WriteTable(buf, output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTable csv error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "Step,Status,Duration,Attempts,Retry,Timeout,Error", "expected CSV header")
	assertContains(t, output, "csv-step", "expected step name in CSV")
	assertContains(t, output, "succeeded", "expected status in CSV")
}

func TestWriteTable_JSON(t *testing.T) {
	t.Parallel()

	a, buf := runSingleSucceedWithBuffer(t, "json-step")

	err := a.Report().WriteTable(buf, output.FormatJSON, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTable json error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "json-step", "expected step name in JSON")
	assertContains(t, output, "succeeded", "expected status in JSON")
}

func TestWriteTable_JSONL(t *testing.T) {
	t.Parallel()

	a, buf := runSingleSucceedWithBuffer(t, "jsonl-step")

	err := a.Report().WriteTable(buf, output.FormatJSONL, output.RenderOptions{})
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

	a := runSingleSucceed(t, "table-string")

	output, err := a.Report().WriteTableString(output.FormatMarkdown, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	assertContains(t, output, "table-string", "expected step name in table string")
}

func TestExportTable(t *testing.T) {
	t.Parallel()

	a, path := singleSucceedExportPath(t, "export-table", "report.csv")

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

	a := runSingleSucceed(t, "tree-string")

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

	a := runSingleSucceed(t, "html-tree-string")

	output, err := a.Report().WriteHTMLTreeString()
	if err != nil {
		t.Fatalf("WriteHTMLTreeString error: %v", err)
	}

	assertContains(t, output, "<ul>", "expected <ul> in HTML tree string")
}

func TestExportTree(t *testing.T) {
	t.Parallel()

	a, path := singleSucceedExportPath(t, "export-tree", "tree.txt")

	err := a.ExportTree(path)
	if err != nil {
		t.Fatalf("ExportTree error: %v", err)
	}
}

func TestExportHTMLTree(t *testing.T) {
	t.Parallel()

	a, path := singleSucceedExportPath(t, "export-html-tree", "tree.html")

	err := a.ExportHTMLTree(path)
	if err != nil {
		t.Fatalf("ExportHTMLTree error: %v", err)
	}
}

// TestWorkflowReport_ExportMethods verifies that a WorkflowReport (e.g. one
// reconstructed via ReplayEvents without an Auditor) can export every format to
// a file. This is the core value of adding Export* to WorkflowReport.
func TestWorkflowReport_ExportMethods(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "report-export")

	report := a.Report()
	dir := t.TempDir()

	for _, tc := range []struct {
		name string
		fn   func(string) error
		ext  string
	}{
		{"JSON", report.ExportJSON, ".json"},
		{"NDJSON", report.ExportNDJSON, ".ndjson"},
		{"Mermaid", func(path string) error { return report.ExportMermaid(path) }, ".mmd"},
		{"PlantUML", func(path string) error { return report.ExportPlantUML(path) }, ".puml"},
		{"Graphviz", func(path string) error { return report.ExportGraphviz(path) }, ".dot"},
		{"D2", func(path string) error { return report.ExportD2(path) }, ".d2"},
		{"Tree", report.ExportTree, ".txt"},
		{"HTMLTree", report.ExportHTMLTree, ".html"},
		{"HTML", report.ExportHTML, ".html"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(dir, strings.ToLower(tc.name)+tc.ext)

			err := tc.fn(path)
			if err != nil {
				t.Fatalf("%s export error: %v", tc.name, err)
			}

			info, statErr := os.Stat(path)
			if statErr != nil {
				t.Fatalf("stat %s: %v", path, statErr)
			}

			if info.Size() == 0 {
				t.Errorf("%s export wrote an empty file", tc.name)
			}
		})
	}
}

// TestAuditor_WriteStringMethods verifies the Auditor.Write*String methods
// mirror their WorkflowReport counterparts and return non-empty output.
func TestAuditor_WriteStringMethods(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "str-methods")

	for _, tc := range []struct {
		name string
		fn   func() (string, error)
	}{
		{"Mermaid", func() (string, error) { return a.WriteMermaidString() }},
		{"PlantUML", func() (string, error) { return a.WritePlantUMLString() }},
		{"Graphviz", func() (string, error) { return a.WriteGraphvizString() }},
		{"D2", func() (string, error) { return a.WriteD2String() }},
		{"Tree", a.WriteTreeString},
		{"HTMLTree", a.WriteHTMLTreeString},
		{"HTML", a.WriteHTMLString},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := tc.fn()
			if err != nil {
				t.Fatalf("%s string error: %v", tc.name, err)
			}

			if out == "" {
				t.Errorf("%s string returned empty output", tc.name)
			}
		})
	}

	// Table string variant needs format + opts.
	tableOut, err := a.WriteTableString(output.FormatMarkdown, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	if tableOut == "" {
		t.Error("WriteTableString returned empty output")
	}
}
