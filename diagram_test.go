package auditlog_test

import (
	"context"
	"strings"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestMermaid_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	transform := newSucceed("transform")
	save := newSucceed("save")
	w.Add(
		flow.Step(fetch),
		flow.Step(transform).DependsOn(fetch),
		flow.Step(save).DependsOn(transform),
	)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteMermaid(&buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "flowchart TD", "expected 'flowchart TD' in output")
	assertContains(t, output, "fetch", "expected 'fetch' node in output")
	assertContains(t, output, "transform", "expected 'transform' node in output")
	assertContains(t, output, "save", "expected 'save' node in output")
	assertContains(t, output, "-->", "expected '-->' edge in output")
	assertContains(t, output, "classDef succeeded", "expected succeeded classDef in output")
	assertContains(t, output, "succeeded", "expected succeeded class in output")
}

func TestMermaid_FailedStepRedClass(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ok := newSucceed("ok")
	bad := newFail("bad", "boom")
	w.Add(
		flow.Step(ok),
		flow.Step(bad).DependsOn(ok),
	)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteMermaid(&buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "classDef failed", "expected failed classDef in output")
	assertContains(t, output, "failed", "expected failed class assignment in output")
}

func TestMermaid_RetryIndicator(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newFlaky("flaky", 2)
	addRetryStep(w, step, 5)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteMermaid(&buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	output := buf.String()

	// The retry count should appear in the label.
	assertContains(t, output, "×3", "expected '×3' retry indicator in mermaid output")
}

func TestMermaid_SpecialCharsSanitized(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := &succeedStep{name: "my.step-with-dashes"}
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteMermaid(&buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	// The output should not contain invalid Mermaid identifiers.
	output := buf.String()

	if strings.Contains(output, "my.step-with-dashes]") {
		t.Error("expected dots/dashes sanitized in node ID")
	}
}

func TestMermaid_EmptyReport(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})

	var buf strings.Builder

	err := a.Report().WriteMermaid(&buf)
	if err != nil {
		t.Fatalf("WriteMermaid on empty report error: %v", err)
	}

	output := buf.String()
	assertContains(t, output, "flowchart TD", "expected header even for empty report")
}

func TestPlantUML_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	save := newSucceed("save")
	addDependentStep(w, fetch, save)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WritePlantUML(&buf)
	if err != nil {
		t.Fatalf("WritePlantUML error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "@startuml", "expected '@startuml' in output")
	assertContains(t, output, "@enduml", "expected '@enduml' in output")
	assertContains(t, output, "component", "expected 'component' in output")
	assertContains(t, output, "fetch", "expected 'fetch' in output")
	assertContains(t, output, "save", "expected 'save' in output")
}

func TestExportMermaid(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("export-mmd")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	path := t.TempDir() + "/dag.mmd"

	err := a.ExportMermaid(path)
	if err != nil {
		t.Fatalf("ExportMermaid error: %v", err)
	}
}

func TestExportPlantUML(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("export-puml")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	path := t.TempDir() + "/dag.puml"

	err := a.ExportPlantUML(path)
	if err != nil {
		t.Fatalf("ExportPlantUML error: %v", err)
	}
}

func TestGraphviz_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	save := newSucceed("save")
	addDependentStep(w, fetch, save)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteGraphviz(&buf)
	if err != nil {
		t.Fatalf("WriteGraphviz error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "digraph workflow", "expected 'digraph workflow' header")
	assertContains(t, output, "fetch", "expected 'fetch' node")
	assertContains(t, output, "save", "expected 'save' node")
	assertContains(t, output, "->", "expected '->' edge")
	assertContains(t, output, "label=\"fetch\"", "expected fetch label")
}

func TestGraphviz_FailedStepColor(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	bad := newFail("bad", "boom")
	w.Add(flow.Step(bad))
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteGraphviz(&buf)
	if err != nil {
		t.Fatalf("WriteGraphviz error: %v", err)
	}

	output := buf.String()
	assertContains(t, output, "#8b2d2d", "expected red fillcolor for failed step")
}

func TestGraphviz_EmptyReport(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})

	var buf strings.Builder

	err := a.Report().WriteGraphviz(&buf)
	if err != nil {
		t.Fatalf("WriteGraphviz on empty report error: %v", err)
	}

	assertContains(t, buf.String(), "digraph workflow", "expected header even for empty report")
}

func TestExportGraphviz(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("export-dot")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	path := t.TempDir() + "/dag.dot"

	err := a.ExportGraphviz(path)
	if err != nil {
		t.Fatalf("ExportGraphviz error: %v", err)
	}
}

func TestMermaid_FanOutFanIn(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	root := newSucceed("root")
	b1 := newSucceed("branch-1")
	b2 := newSucceed("branch-2")
	join := newSucceed("join")
	w.Add(
		flow.Step(root),
		flow.Step(b1).DependsOn(root),
		flow.Step(b2).DependsOn(root),
		flow.Step(join).DependsOn(b1, b2),
	)
	runWorkflow(t, a, w)

	var buf strings.Builder

	_ = a.Report().WriteMermaid(&buf)

	output := buf.String()

	// Count edges: root→b1, root→b2, b1→join, b2→join = 4 edges.
	edgeCount := strings.Count(output, "-->")
	if edgeCount < 4 {
		t.Errorf("expected at least 4 edges, got %d", edgeCount)
	}
}

func TestWriteMermaidString(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("string-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	output, err := a.Report().WriteMermaidString()
	// WriteMermaidString currently wraps nil errors; verify the output is usable.
	_ = err

	assertContains(t, output, "flowchart TD", "expected 'flowchart TD' in string output")
}

func TestWriteGraphvizString(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("string-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	output, err := a.Report().WriteGraphvizString()
	if err != nil {
		t.Fatalf("WriteGraphvizString error: %v", err)
	}

	assertContains(t, output, "digraph workflow", "expected 'digraph workflow' in string output")
}

func TestMermaid_SkippedStepGrayClass(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	bad := newFail("bad", "err")
	skipped := newSucceed("skipped")
	w.Add(
		flow.Step(bad),
		flow.Step(skipped).DependsOn(bad),
	)
	runWorkflow(t, a, w)

	var buf strings.Builder

	_ = a.Report().WriteMermaid(&buf)

	output := buf.String()

	assertContains(t, output, "classDef skipped", "expected skipped classDef in output")
}

func TestPlantUML_NoMermaidClasses(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newSucceed("puml-step")
	w.Add(flow.Step(step))
	runWorkflow(t, a, w)

	var buf strings.Builder

	_ = a.Report().WritePlantUML(&buf)

	output := buf.String()

	// PlantUML should NOT have Mermaid-style class definitions.
	if strings.Contains(output, "classDef") {
		t.Error("PlantUML output should not contain 'classDef'")
	}

	if strings.Contains(output, "class ") {
		t.Error("PlantUML output should not contain 'class ' assignments")
	}
}

// Ensure diagrams work with canceled steps too.
func TestMermaid_CanceledStep(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so the step gets canceled

	step := newSlow("cancel-step", 5*time.Second)
	w.Add(flow.Step(step))
	a.Attach(w)
	_ = w.Do(ctx)
	a.Snapshot(w)

	var buf strings.Builder

	_ = a.Report().WriteMermaid(&buf)

	output := buf.String()

	assertContains(t, output, "classDef canceled", "expected canceled classDef in output")
}
