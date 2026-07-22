package viz_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

// --- D2 Diagram Tests ---

// renderMarkdownTable renders the report's step summary as Markdown into a
// buffer and returns it. Centralizes the
// `var buf strings.Builder; err := viz.WriteTable(a.Report(), &buf, output.FormatMarkdown, output.RenderOptions{})`
// boilerplate used by output tests that need a Markdown rendering of a report.
func renderMarkdownTable(t *testing.T, a *auditlog.Auditor) string {
	t.Helper()

	var buf strings.Builder

	err := viz.WriteTable(a.Report(), &buf, output.FormatMarkdown, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTable markdown error: %v", err)
	}

	return buf.String()
}

// --- D2 Diagram Tests ---

func TestD2_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	transform := testhelpers.NewSucceed("transform")
	save := testhelpers.NewSucceed("save")

	testhelpers.AddDependentStep(w, fetch, transform)
	testhelpers.AddDependentStep(w, transform, save)
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WriteD2(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	output := buf.String()

	testhelpers.AssertContains(t, output, "title:", "expected title block in D2 output")
	testhelpers.AssertContains(t, output, "test", "expected D2 title to derive from WorkflowID 'test'")
	testhelpers.AssertContains(t, output, "fetch", "expected 'fetch' node in output")
	testhelpers.AssertContains(t, output, "transform", "expected 'transform' node in output")
	testhelpers.AssertContains(t, output, "save", "expected 'save' node in output")
	testhelpers.AssertContains(t, output, "->", "expected '->' edge in output")
	testhelpers.AssertContains(t, output, "#2d5a2d", "expected green fill color for succeeded step")
}

func TestD2_FailedStepRedColor(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	bad := testhelpers.NewFail("bad", "boom")
	w.Add(flow.Step(bad))
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WriteD2(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	output := buf.String()
	testhelpers.AssertContains(t, output, "#8b2d2d", "expected red fill color for failed step")
}

func TestD2_EmptyReport(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	var buf strings.Builder

	err := viz.WriteD2(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteD2 on empty report error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "title:", "expected title block even for empty report")
}

func TestWriteD2String(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "d2-string")

	output, err := viz.WriteD2String(a.Report())
	if err != nil {
		t.Fatalf("WriteD2String error: %v", err)
	}

	testhelpers.AssertContains(t, output, "title:", "expected 'title:' in string output")
}

func TestExportD2(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "export-d2", "dag.d2")

	err := viz.ExportD2(a.Report(), path)
	if err != nil {
		t.Fatalf("ExportD2 error: %v", err)
	}
}

// --- Table Tests ---

func TestWriteTable_Markdown(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	bad := testhelpers.NewFail("bad", "boom")
	w.Add(flow.Step(fetch), flow.Step(bad))
	testhelpers.RunWorkflow(t, a, w)

	output := renderMarkdownTable(t, a)

	testhelpers.AssertContains(t, output, "Step", "expected markdown table header")
	testhelpers.AssertContains(t, output, "fetch", "expected fetch row")
	testhelpers.AssertContains(t, output, "bad", "expected bad row")
	testhelpers.AssertContains(t, output, "succeeded", "expected succeeded status")
	testhelpers.AssertContains(t, output, "failed", "expected failed status")
}

func TestWriteTable_CSV(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "csv-step")

	err := viz.WriteTable(a.Report(), buf, output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTable csv error: %v", err)
	}

	output := buf.String()

	testhelpers.AssertContains(t, output, "Step,Status,Duration,Attempts,Retry,Timeout,Error", "expected CSV header")
	testhelpers.AssertContains(t, output, "csv-step", "expected step name in CSV")
	testhelpers.AssertContains(t, output, "succeeded", "expected status in CSV")
}

func TestWriteTable_JSON(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "json-step")

	err := viz.WriteTable(a.Report(), buf, output.FormatJSON, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTable json error: %v", err)
	}

	output := buf.String()

	testhelpers.AssertContains(t, output, "json-step", "expected step name in JSON")
	testhelpers.AssertContains(t, output, "succeeded", "expected status in JSON")
}

func TestWriteTable_JSONL(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "jsonl-step")

	err := viz.WriteTable(a.Report(), buf, output.FormatJSONL, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTable jsonl error: %v", err)
	}

	output := buf.String()

	// JSONL: one object per line.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 JSONL line, got %d", len(lines))
	}

	testhelpers.AssertContains(t, output, "jsonl-step", "expected step name in JSONL")
}

func TestWriteTable_EmptyReport(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	output := renderMarkdownTable(t, a)
	testhelpers.AssertContains(t, output, "| Step |", "expected header even for empty report")
}

func TestWriteTableString(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "table-string")

	output, err := viz.WriteTableString(a.Report(), output.FormatMarkdown, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	testhelpers.AssertContains(t, output, "table-string", "expected step name in table string")
}

func TestExportTable(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "export-table", "report.csv")

	err := viz.ExportTable(a.Report(), path, output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("ExportTable error: %v", err)
	}
}

// --- Tree Tests ---

func TestWriteTree_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	transform := testhelpers.NewSucceed("transform")
	testhelpers.AddDependentStep(w, fetch, transform)
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WriteTree(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteTree error: %v", err)
	}

	output := buf.String()

	testhelpers.AssertContains(t, output, "Workflow", "expected root label")
	testhelpers.AssertContains(t, output, "fetch", "expected fetch in tree")
	testhelpers.AssertContains(t, output, "transform", "expected transform in tree")
}

func TestWriteTree_EmptyReport(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	var buf strings.Builder

	err := viz.WriteTree(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteTree on empty report error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "Workflow", "expected root label even for empty report")
}

func TestWriteTreeString(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "tree-string")

	output, err := viz.WriteTreeString(a.Report())
	if err != nil {
		t.Fatalf("WriteTreeString error: %v", err)
	}

	testhelpers.AssertContains(t, output, "tree-string", "expected step name in tree string")
}

func TestWriteHTMLTree_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	transform := testhelpers.NewSucceed("transform")
	testhelpers.AddDependentStep(w, fetch, transform)
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WriteHTMLTree(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteHTMLTree error: %v", err)
	}

	output := buf.String()

	testhelpers.AssertContains(t, output, "<ul>", "expected <ul> in HTML tree")
	testhelpers.AssertContains(t, output, "fetch", "expected fetch in HTML tree")
	testhelpers.AssertContains(t, output, "transform", "expected transform in HTML tree")
}

func TestWriteHTMLTreeString(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "html-tree-string")

	output, err := viz.WriteHTMLTreeString(a.Report())
	if err != nil {
		t.Fatalf("WriteHTMLTreeString error: %v", err)
	}

	testhelpers.AssertContains(t, output, "<ul>", "expected <ul> in HTML tree string")
}

func TestExportTree(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "export-tree", "tree.txt")

	err := viz.ExportTree(a.Report(), path)
	if err != nil {
		t.Fatalf("ExportTree error: %v", err)
	}
}

func TestExportHTMLTree(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "export-html-tree", "tree.html")

	err := viz.ExportHTMLTree(a.Report(), path)
	if err != nil {
		t.Fatalf("ExportHTMLTree error: %v", err)
	}
}

// TestWorkflowReport_ExportMethods verifies that a WorkflowReport (e.g. one
// reconstructed via ReplayEvents without an Auditor) can export every format to
// a file. This is the core value of adding Export* to WorkflowReport.
func TestWorkflowReport_ExportMethods(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "report-export")

	report := a.Report()
	dir := t.TempDir()

	for _, tc := range []struct {
		name string
		fn   func(string) error
		ext  string
	}{
		{"JSON", report.ExportJSON, ".json"},
		{"NDJSON", report.ExportNDJSON, ".ndjson"},
		{"Mermaid", func(path string) error { return viz.ExportMermaid(report, path) }, ".mmd"},
		{"PlantUML", func(path string) error { return viz.ExportPlantUML(report, path) }, ".puml"},
		{"Graphviz", func(path string) error { return viz.ExportGraphviz(report, path) }, ".dot"},
		{"D2", func(path string) error { return viz.ExportD2(report, path) }, ".d2"},
		{"Tree", func(path string) error { return viz.ExportTree(report, path) }, ".txt"},
		{"HTMLTree", func(path string) error { return viz.ExportHTMLTree(report, path) }, ".html"},
		{"HTML", func(path string) error { return viz.ExportHTML(report, path) }, ".html"},
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

	a := testhelpers.RunSingleSucceed(t, "str-methods")

	for _, tc := range []struct {
		name string
		fn   func() (string, error)
	}{
		{"Mermaid", func() (string, error) { return viz.WriteMermaidString(a.Report()) }},
		{"PlantUML", func() (string, error) { return viz.WritePlantUMLString(a.Report()) }},
		{"Graphviz", func() (string, error) { return viz.WriteGraphvizString(a.Report()) }},
		{"D2", func() (string, error) { return viz.WriteD2String(a.Report()) }},
		{"Tree", func() (string, error) { return viz.WriteTreeString(a.Report()) }},
		{"HTMLTree", func() (string, error) { return viz.WriteHTMLTreeString(a.Report()) }},
		{"HTML", func() (string, error) { return viz.WriteHTMLString(a.Report()) }},
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
	tableOut, err := viz.WriteTableString(a.Report(), output.FormatMarkdown, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	if tableOut == "" {
		t.Error("WriteTableString returned empty output")
	}
}
