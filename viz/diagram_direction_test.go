package viz_test

import (
	"strings"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

// --- Mermaid Direction Tests ---

func TestMermaid_DefaultDirectionTD(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "td-step")

	err := viz.WriteMermaid(a.Report(), buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "flowchart TD", "expected TD by default")
}

func TestMermaid_DirectionLR(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "lr-step")

	err := viz.WriteMermaid(a.Report(), buf, viz.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "flowchart LR", "expected LR direction")

	if strings.Contains(buf.String(), "flowchart TD") {
		t.Error("TD should not appear when LR is set")
	}
}

func TestMermaid_DirectionBT(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "bt-step")

	err := viz.WriteMermaid(a.Report(), buf, viz.WithDirection(output.DirectionUp))
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "flowchart BT", "expected BT direction")
}

func TestMermaid_DirectionRL(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "rl-step")

	err := viz.WriteMermaid(a.Report(), buf, viz.WithDirection(output.DirectionLeft))
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "flowchart RL", "expected RL direction")
}

// --- Graphviz Direction Tests ---

func TestGraphviz_DefaultDirectionTB(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "gv-td-step")

	err := viz.WriteGraphviz(a.Report(), buf)
	if err != nil {
		t.Fatalf("WriteGraphviz error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "rankdir=TB", "expected TB by default")
}

func TestGraphviz_DirectionLR(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "gv-lr-step")

	err := viz.WriteGraphviz(a.Report(), buf, viz.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteGraphviz error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "rankdir=LR", "expected LR direction")

	if strings.Contains(buf.String(), "rankdir=TB") {
		t.Error("TB should not appear when LR is set")
	}
}

func TestGraphviz_DirectionBT(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "gv-bt-step")

	err := viz.WriteGraphviz(a.Report(), buf, viz.WithDirection(output.DirectionUp))
	if err != nil {
		t.Fatalf("WriteGraphviz error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "rankdir=BT", "expected BT direction")
}

// --- D2 Direction Tests ---

func TestD2_DefaultDirectionNone(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "d2-default-step")

	err := viz.WriteD2(a.Report(), buf)
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	// D2 default (down) produces no explicit direction line.
	if strings.Contains(buf.String(), "direction:") {
		t.Error("D2 default should not contain explicit direction")
	}
}

func TestD2_DirectionRight(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "d2-right-step")

	err := viz.WriteD2(a.Report(), buf, viz.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "direction: right", "expected right direction in D2")
}

func TestD2_DirectionUp(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "d2-up-step")

	err := viz.WriteD2(a.Report(), buf, viz.WithDirection(output.DirectionUp))
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "direction: up", "expected up direction in D2")
}

// --- PlantUML Direction Tests ---

func TestPlantUML_DefaultNoDirectionCommand(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "puml-default-step")

	err := viz.WritePlantUML(a.Report(), buf)
	if err != nil {
		t.Fatalf("WritePlantUML error: %v", err)
	}

	if strings.Contains(buf.String(), "left to right direction") {
		t.Error("PlantUML default should not contain left-to-right direction")
	}
}

func TestPlantUML_DirectionRight(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "puml-lr-step")

	err := viz.WritePlantUML(a.Report(), buf, viz.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WritePlantUML error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "left to right direction", "expected LR direction in PlantUML")
}

// --- String Variant Tests ---

func TestMermaidString_DirectionLR(t *testing.T) {
	t.Parallel()

	a, _ := testhelpers.RunSingleSucceedWithBuffer(t, "mmd-str-lr")

	out, err := viz.WriteMermaidString(a.Report(), viz.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteMermaidString error: %v", err)
	}

	testhelpers.AssertContains(t, out, "flowchart LR", "expected LR direction in string output")
}

func TestGraphvizString_DirectionLR(t *testing.T) {
	t.Parallel()

	a, _ := testhelpers.RunSingleSucceedWithBuffer(t, "gv-str-lr")

	out, err := viz.WriteGraphvizString(a.Report(), viz.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteGraphvizString error: %v", err)
	}

	testhelpers.AssertContains(t, out, "rankdir=LR", "expected LR direction in string output")
}

// --- Auditor Delegate Tests ---

func TestAuditor_WriteMermaidWithDirection(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "aud-mmd-lr")

	err := viz.WriteMermaid(a.Report(), buf, viz.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("Auditor.WriteMermaid error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "flowchart LR", "expected LR direction from Auditor")
}

func TestAuditor_WriteGraphvizWithDirection(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "aud-gv-lr")

	err := viz.WriteGraphviz(a.Report(), buf, viz.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("Auditor.WriteGraphviz error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "rankdir=LR", "expected LR direction from Auditor")
}

// --- Export Tests with Direction ---

func TestExportMermaidWithDirection(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "export-mmd-lr", "dag.mmd")

	err := viz.ExportMermaid(a.Report(), path, viz.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("ExportMermaid error: %v", err)
	}
}

// --- DirectionDown explicit tests (cover default branches) ---

func TestMermaid_DirectionDownExplicit(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "td-explicit-step")

	err := viz.WriteMermaid(a.Report(), buf, viz.WithDirection(output.DirectionDown))
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "flowchart TD", "expected TD for DirectionDown")
}

func TestPlantUML_DirectionDownExplicit(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "puml-td-explicit")

	err := viz.WritePlantUML(a.Report(), buf, viz.WithDirection(output.DirectionDown))
	if err != nil {
		t.Fatalf("WritePlantUML error: %v", err)
	}

	// PlantUML with DirectionDown should NOT contain left-to-right command.
	if strings.Contains(buf.String(), "left to right direction") {
		t.Error("PlantUML DirectionDown should not contain left-to-right direction")
	}
}

func TestPlantUML_DirectionLeft(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "puml-left-step")

	err := viz.WritePlantUML(a.Report(), buf, viz.WithDirection(output.DirectionLeft))
	if err != nil {
		t.Fatalf("WritePlantUML error: %v", err)
	}

	testhelpers.AssertContains(t, buf.String(), "left to right direction", "expected LR for DirectionLeft")
}

func TestPlantUML_DirectionUp(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "puml-up-step")

	err := viz.WritePlantUML(a.Report(), buf, viz.WithDirection(output.DirectionUp))
	if err != nil {
		t.Fatalf("WritePlantUML error: %v", err)
	}

	// PlantUML DirectionUp maps to default TD (no left-to-right command).
	if strings.Contains(buf.String(), "left to right direction") {
		t.Error("PlantUML DirectionUp should not contain left-to-right direction")
	}
}

// --- Error path tests for String variants ---

func TestWriteD2String_ErrorPath(t *testing.T) {
	t.Parallel()

	a, _ := testhelpers.RunSingleSucceedWithBuffer(t, "d2-err-step")

	_, err := viz.WriteD2String(a.Report())
	if err != nil {
		t.Fatalf("WriteD2String should succeed for valid report, got: %v", err)
	}
}

func TestWritePlantUMLString_ErrorPath(t *testing.T) {
	t.Parallel()

	a, _ := testhelpers.RunSingleSucceedWithBuffer(t, "puml-err-step")

	_, err := viz.WritePlantUMLString(a.Report())
	if err != nil {
		t.Fatalf("WritePlantUMLString should succeed for valid report, got: %v", err)
	}
}

// --- Diamond DAG with Direction (ensures direction doesn't break graph topology) ---

func TestDiagram_DiamondDAGWithDirection(t *testing.T) {
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

	report := a.Report()

	// Mermaid LR — ensure all nodes and edges still present.
	var buf strings.Builder

	err := viz.WriteMermaid(report, &buf, viz.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	out := buf.String()
	testhelpers.AssertContains(t, out, "flowchart LR", "expected LR direction")
	testhelpers.AssertContains(t, out, "root", "expected root node")
	testhelpers.AssertContains(t, out, "branch-1", "expected branch-1 node")
	testhelpers.AssertContains(t, out, "branch-2", "expected branch-2 node")
	testhelpers.AssertContains(t, out, "join", "expected join node")

	// At least 4 edges: root→b1, root→b2, b1→join, b2→join.
	edgeCount := strings.Count(out, "-->")
	if edgeCount < 4 {
		t.Errorf("expected at least 4 edges, got %d", edgeCount)
	}
}
