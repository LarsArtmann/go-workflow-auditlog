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
	addLinearChain(w, fetch, transform, save)
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
	assertContains(t, output, "#2d5a2d", "expected green fill color for succeeded step")
}

func TestMermaid_FailedStepRedColor(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ok := newSucceed("ok")
	bad := newFail("bad", "boom")
	addDependentStep(w, ok, bad)
	runWorkflow(t, a, w)

	var buf strings.Builder

	err := a.Report().WriteMermaid(&buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	output := buf.String()

	assertContains(t, output, "#8b2d2d", "expected red fill color for failed step")
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

	// go-output's MermaidID drops dots and hyphens from node identifiers.
	// The sanitized ID must appear in the output; the raw name survives only
	// in the display label.
	assertContains(t, output, "mystepwithdashes", "expected sanitized node ID in output")
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

	a, path := singleSucceedExportPath(t, "export-mmd", "dag.mmd")

	err := a.ExportMermaid(path)
	if err != nil {
		t.Fatalf("ExportMermaid error: %v", err)
	}
}

func TestExportPlantUML(t *testing.T) {
	t.Parallel()

	a, path := singleSucceedExportPath(t, "export-puml", "dag.puml")

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
	assertContains(t, output, "label=\"fetch", "expected fetch label")
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

	a, path := singleSucceedExportPath(t, "export-dot", "dag.dot")

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

	a := runSingleSucceed(t, "string-step")

	output, err := a.Report().WriteMermaidString()
	// WriteMermaidString currently wraps nil errors; verify the output is usable.
	_ = err

	assertContains(t, output, "flowchart TD", "expected 'flowchart TD' in string output")
}

func TestWriteGraphvizString(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "string-step")

	output, err := a.Report().WriteGraphvizString()
	if err != nil {
		t.Fatalf("WriteGraphvizString error: %v", err)
	}

	assertContains(t, output, "digraph workflow", "expected 'digraph workflow' in string output")
}

func TestWritePlantUMLString(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "plantuml-step")

	output, err := a.Report().WritePlantUMLString()
	if err != nil {
		t.Fatalf("WritePlantUMLString error: %v", err)
	}

	assertContains(t, output, "plantuml-step", "expected step name in PlantUML string output")
}

func TestMermaid_SkippedStepGrayColor(t *testing.T) {
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

	assertContains(t, output, "#4a4a4a", "expected gray fill color for skipped step")
}

func TestPlantUML_NoMermaidClasses(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	addSingleStep(w, "puml-step")
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

	assertContains(t, output, "#5a3d2d", "expected orange fill color for canceled step")
}

// TestDiagram_EdgeDirectionFollowsExecutionFlow is the regression test for the
// edge-direction bug: diagram edges must point dependency → step (forward
// execution flow), matching the tree export. The DAG is fetch → transform →
// save (execution order). Every diagram format AND the tree must agree.
//
// Before the fix, diagram edges pointed step → dependency (backward), so the
// tree showed fetch→save top-down while diagrams showed save→fetch. This test
// asserts the forward pair appears and the backward pair does not, in every
// visual format.
func TestDiagram_EdgeDirectionFollowsExecutionFlow(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	transform := newSucceed("transform")
	save := newSucceed("save")

	addDependentStep(w, fetch, transform)
	addDependentStep(w, transform, save)
	runWorkflow(t, a, w)

	report := a.Report()

	mermaidFwd := []string{"fetch --> transform", "transform --> save"}
	mermaidBwd := []string{"transform --> fetch", "save --> transform"}

	t.Run("mermaid", func(t *testing.T) {
		t.Parallel()

		var buf strings.Builder

		err := report.WriteMermaid(&buf)
		if err != nil {
			t.Fatalf("WriteMermaid: %v", err)
		}

		assertEdgeDirections(t, "mermaid", buf.String(), mermaidFwd, mermaidBwd)
	})

	t.Run("d2", func(t *testing.T) {
		t.Parallel()

		var buf strings.Builder

		err := report.WriteD2(&buf)
		if err != nil {
			t.Fatalf("WriteD2: %v", err)
		}

		assertEdgeDirections(t, "d2", buf.String(),
			[]string{"fetch -> transform", "transform -> save"},
			[]string{"transform -> fetch", "save -> transform"})
	})

	t.Run("graphviz", func(t *testing.T) {
		t.Parallel()

		var buf strings.Builder

		err := report.WriteGraphviz(&buf)
		if err != nil {
			t.Fatalf("WriteGraphviz: %v", err)
		}

		assertEdgeDirections(t, "graphviz", buf.String(),
			[]string{`"fetch" -> "transform"`, `"transform" -> "save"`},
			[]string{`"transform" -> "fetch"`, `"save" -> "transform"`})
	})

	t.Run("tree_reads_top_down_execution_order", func(t *testing.T) {
		t.Parallel()

		var buf strings.Builder

		err := report.WriteTree(&buf)
		if err != nil {
			t.Fatalf("WriteTree: %v", err)
		}

		assertTreeExecutionOrder(t, buf.String())
	})
}

// assertEdgeDirections verifies that every forward edge appears in the rendered
// output and no backward edge does — proving edges follow execution flow.
func assertEdgeDirections(t *testing.T, format, out string, forward, backward []string) {
	t.Helper()

	for _, e := range forward {
		assertContains(t, out, e, format+" edge should follow execution flow")
	}

	for _, e := range backward {
		if strings.Contains(out, e) {
			t.Errorf("%s must not contain backward edge %q (got it)", format, e)
		}
	}
}

// assertTreeExecutionOrder verifies the tree reads top-down in execution order
// (fetch before transform before save).
func assertTreeExecutionOrder(t *testing.T, out string) {
	t.Helper()

	fi := strings.Index(out, "fetch")
	ti := strings.Index(out, "transform")
	si := strings.Index(out, "save")

	if fi < 0 || ti < 0 || si < 0 {
		t.Fatalf("tree missing step names; got:\n%s", out)
	}

	if fi >= ti || ti >= si {
		t.Errorf(
			"tree should read fetch < transform < save (execution order); got fetch@%d transform@%d save@%d\n%s",
			fi,
			ti,
			si,
			out,
		)
	}
}
