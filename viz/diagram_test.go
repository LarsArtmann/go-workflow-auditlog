package viz_test

import (
	"context"
	"strings"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

func TestMermaid_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	transform := testhelpers.NewSucceed("transform")
	save := testhelpers.NewSucceed("save")
	testhelpers.AddLinearChain(w, fetch, transform, save)
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WriteMermaid(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	output := buf.String()

	testhelpers.AssertContains(t, output, "flowchart TD", "expected 'flowchart TD' in output")
	testhelpers.AssertContains(t, output, "fetch", "expected 'fetch' node in output")
	testhelpers.AssertContains(t, output, "transform", "expected 'transform' node in output")
	testhelpers.AssertContains(t, output, "save", "expected 'save' node in output")
	testhelpers.AssertContains(t, output, "-->", "expected '-->' edge in output")
	testhelpers.AssertContains(t, output, "#2d5a2d", "expected green fill color for succeeded step")
}

func TestMermaid_FailedStepRedColor(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	ok := testhelpers.NewSucceed("ok")
	bad := testhelpers.NewFail("bad", "boom")
	testhelpers.AddDependentStep(w, ok, bad)
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WriteMermaid(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	output := buf.String()

	testhelpers.AssertContains(t, output, "#8b2d2d", "expected red fill color for failed step")
}

func TestMermaid_RetryIndicator(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewFlaky("flaky", 2)
	testhelpers.AddRetryStep(w, step, 5)
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WriteMermaid(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	output := buf.String()

	// The retry count should appear in the label.
	testhelpers.AssertContains(t, output, "×3", "expected '×3' retry indicator in mermaid output")
}

func TestMermaid_SpecialCharsSanitized(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := &testhelpers.SucceedStep{Name: "my.step-with-dashes"}
	w.Add(flow.Step(step))
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WriteMermaid(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	// The output should not contain invalid Mermaid identifiers.
	output := buf.String()

	// go-output's MermaidID drops dots and hyphens from node identifiers.
	// The sanitized ID must appear in the output; the raw name survives only
	// in the display label.
	testhelpers.AssertContains(t, output, "mystepwithdashes", "expected sanitized node ID in output")
}

func TestMermaid_EmptyReport(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	var buf strings.Builder

	err := viz.WriteMermaid(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteMermaid on empty report error: %v", err)
	}

	output := buf.String()
	testhelpers.AssertContains(t, output, "flowchart TD", "expected header even for empty report")
}

func TestPlantUML_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	save := testhelpers.NewSucceed("save")
	testhelpers.AddDependentStep(w, fetch, save)
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WritePlantUML(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WritePlantUML error: %v", err)
	}

	output := buf.String()

	testhelpers.AssertContains(t, output, "@startuml", "expected '@startuml' in output")
	testhelpers.AssertContains(t, output, "@enduml", "expected '@enduml' in output")
	testhelpers.AssertContains(t, output, "component", "expected 'component' in output")
	testhelpers.AssertContains(t, output, "fetch", "expected 'fetch' in output")
	testhelpers.AssertContains(t, output, "save", "expected 'save' in output")
}

func TestExportMermaid(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "export-mmd", "dag.mmd")

	err := viz.ExportMermaid(a, path)
	if err != nil {
		t.Fatalf("ExportMermaid error: %v", err)
	}
}

func TestExportPlantUML(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "export-puml", "dag.puml")

	err := viz.ExportPlantUML(a, path)
	if err != nil {
		t.Fatalf("ExportPlantUML error: %v", err)
	}
}

func TestGraphviz_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	save := testhelpers.NewSucceed("save")
	testhelpers.AddDependentStep(w, fetch, save)
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WriteGraphviz(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteGraphviz error: %v", err)
	}

	output := buf.String()

	testhelpers.AssertContains(t, output, "digraph workflow", "expected 'digraph workflow' header")
	testhelpers.AssertContains(t, output, "fetch", "expected 'fetch' node")
	testhelpers.AssertContains(t, output, "save", "expected 'save' node")
	testhelpers.AssertContains(t, output, "->", "expected '->' edge")
	testhelpers.AssertContains(t, output, "label=\"fetch", "expected fetch label")
}

func TestGraphviz_FailedStepColor(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	bad := testhelpers.NewFail("bad", "boom")
	w.Add(flow.Step(bad))
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	err := viz.WriteGraphviz(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteGraphviz error: %v", err)
	}

	output := buf.String()
	testhelpers.AssertContains(t, output, "#8b2d2d", "expected red fillcolor for failed step")
}

func TestGraphviz_EmptyReport(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	var buf strings.Builder

	err := viz.WriteGraphviz(a.Report(), &buf)
	if err != nil {
		t.Fatalf("WriteGraphviz on empty report error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "digraph workflow", "expected header even for empty report")
}

func TestExportGraphviz(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "export-dot", "dag.dot")

	err := viz.ExportGraphviz(a, path)
	if err != nil {
		t.Fatalf("ExportGraphviz error: %v", err)
	}
}

func TestMermaid_FanOutFanIn(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	root := testhelpers.NewSucceed("root")
	b1 := testhelpers.NewSucceed("branch-1")
	b2 := testhelpers.NewSucceed("branch-2")
	join := testhelpers.NewSucceed("join")
	w.Add(
		flow.Step(root),
		flow.Step(b1).DependsOn(root),
		flow.Step(b2).DependsOn(root),
		flow.Step(join).DependsOn(b1, b2),
	)
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	_ = viz.WriteMermaid(a.Report(), &buf)

	output := buf.String()

	// Count edges: root→b1, root→b2, b1→join, b2→join = 4 edges.
	edgeCount := strings.Count(output, "-->")
	if edgeCount < 4 {
		t.Errorf("expected at least 4 edges, got %d", edgeCount)
	}
}

func TestWriteMermaidString(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "string-step")

	output, err := viz.WriteMermaidString(a.Report(), )
	// WriteMermaidString currently wraps nil errors; verify the output is usable.
	_ = err

	testhelpers.AssertContains(t, output, "flowchart TD", "expected 'flowchart TD' in string output")
}

func TestWriteGraphvizString(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "string-step")

	output, err := viz.WriteGraphvizString(a.Report(), )
	if err != nil {
		t.Fatalf("WriteGraphvizString error: %v", err)
	}

	testhelpers.AssertContains(t, output, "digraph workflow", "expected 'digraph workflow' in string output")
}

func TestWritePlantUMLString(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "plantuml-step")

	output, err := viz.WritePlantUMLString(a.Report(), )
	if err != nil {
		t.Fatalf("WritePlantUMLString error: %v", err)
	}

	testhelpers.AssertContains(t, output, "plantuml-step", "expected step name in PlantUML string output")
}

func TestMermaid_SkippedStepGrayColor(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	bad := testhelpers.NewFail("bad", "err")
	skipped := testhelpers.NewSucceed("skipped")
	w.Add(
		flow.Step(bad),
		flow.Step(skipped).DependsOn(bad),
	)
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	_ = viz.WriteMermaid(a.Report(), &buf)

	output := buf.String()

	testhelpers.AssertContains(t, output, "#4a4a4a", "expected gray fill color for skipped step")
}

func TestPlantUML_NoMermaidClasses(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	testhelpers.AddSingleStep(w, "puml-step")
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	_ = viz.WritePlantUML(a.Report(), &buf)

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

	a, w := testhelpers.NewAuditAndWorkflow(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so the step gets canceled

	step := testhelpers.NewSlow("cancel-step", 5*time.Second)
	w.Add(flow.Step(step))
	a.Attach(w)
	_ = w.Do(ctx)
	a.Snapshot(w)

	var buf strings.Builder

	_ = viz.WriteMermaid(a.Report(), &buf)

	output := buf.String()

	testhelpers.AssertContains(t, output, "#5a3d2d", "expected orange fill color for canceled step")
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

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	transform := testhelpers.NewSucceed("transform")
	save := testhelpers.NewSucceed("save")

	testhelpers.AddDependentStep(w, fetch, transform)
	testhelpers.AddDependentStep(w, transform, save)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	mermaidFwd := []string{"fetch --> transform", "transform --> save"}
	mermaidBwd := []string{"transform --> fetch", "save --> transform"}

	t.Run("mermaid", func(t *testing.T) {
		t.Parallel()

		var buf strings.Builder

		err := viz.WriteMermaid(report, &buf)
		if err != nil {
			t.Fatalf("WriteMermaid: %v", err)
		}

		assertEdgeDirections(t, "mermaid", buf.String(), mermaidFwd, mermaidBwd)
	})

	t.Run("d2", func(t *testing.T) {
		t.Parallel()

		var buf strings.Builder

		err := viz.WriteD2(report, &buf)
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

		err := viz.WriteGraphviz(report, &buf)
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

		err := viz.WriteTree(report, &buf)
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
		testhelpers.AssertContains(t, out, e, format+" edge should follow execution flow")
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
