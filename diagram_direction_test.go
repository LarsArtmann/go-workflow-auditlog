package auditlog_test

import (
	"strings"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// --- Mermaid Direction Tests ---

func TestMermaid_DefaultDirectionTD(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "td-step")

	var buf strings.Builder

	err := a.Report().WriteMermaid(&buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	assertContains(t, buf.String(), "flowchart TD", "expected TD by default")
}

func TestMermaid_DirectionLR(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "lr-step")

	var buf strings.Builder

	err := a.Report().WriteMermaid(&buf, auditlog.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	assertContains(t, buf.String(), "flowchart LR", "expected LR direction")

	if strings.Contains(buf.String(), "flowchart TD") {
		t.Error("TD should not appear when LR is set")
	}
}

func TestMermaid_DirectionBT(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "bt-step")

	var buf strings.Builder

	err := a.Report().WriteMermaid(&buf, auditlog.WithDirection(output.DirectionUp))
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	assertContains(t, buf.String(), "flowchart BT", "expected BT direction")
}

func TestMermaid_DirectionRL(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "rl-step")

	var buf strings.Builder

	err := a.Report().WriteMermaid(&buf, auditlog.WithDirection(output.DirectionLeft))
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	assertContains(t, buf.String(), "flowchart RL", "expected RL direction")
}

// --- Graphviz Direction Tests ---

func TestGraphviz_DefaultDirectionTB(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "gv-td-step")

	var buf strings.Builder

	err := a.Report().WriteGraphviz(&buf)
	if err != nil {
		t.Fatalf("WriteGraphviz error: %v", err)
	}

	assertContains(t, buf.String(), "rankdir=TB", "expected TB by default")
}

func TestGraphviz_DirectionLR(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "gv-lr-step")

	var buf strings.Builder

	err := a.Report().WriteGraphviz(&buf, auditlog.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteGraphviz error: %v", err)
	}

	assertContains(t, buf.String(), "rankdir=LR", "expected LR direction")

	if strings.Contains(buf.String(), "rankdir=TB") {
		t.Error("TB should not appear when LR is set")
	}
}

func TestGraphviz_DirectionBT(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "gv-bt-step")

	var buf strings.Builder

	err := a.Report().WriteGraphviz(&buf, auditlog.WithDirection(output.DirectionUp))
	if err != nil {
		t.Fatalf("WriteGraphviz error: %v", err)
	}

	assertContains(t, buf.String(), "rankdir=BT", "expected BT direction")
}

// --- D2 Direction Tests ---

func TestD2_DefaultDirectionNone(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "d2-default-step")

	var buf strings.Builder

	err := a.Report().WriteD2(&buf)
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

	a := runSingleSucceed(t, "d2-right-step")

	var buf strings.Builder

	err := a.Report().WriteD2(&buf, auditlog.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	assertContains(t, buf.String(), "direction: right", "expected right direction in D2")
}

func TestD2_DirectionUp(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "d2-up-step")

	var buf strings.Builder

	err := a.Report().WriteD2(&buf, auditlog.WithDirection(output.DirectionUp))
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	assertContains(t, buf.String(), "direction: up", "expected up direction in D2")
}

// --- PlantUML Direction Tests ---

func TestPlantUML_DefaultNoDirectionCommand(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "puml-default-step")

	var buf strings.Builder

	err := a.Report().WritePlantUML(&buf)
	if err != nil {
		t.Fatalf("WritePlantUML error: %v", err)
	}

	if strings.Contains(buf.String(), "left to right direction") {
		t.Error("PlantUML default should not contain left-to-right direction")
	}
}

func TestPlantUML_DirectionRight(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "puml-lr-step")

	var buf strings.Builder

	err := a.Report().WritePlantUML(&buf, auditlog.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WritePlantUML error: %v", err)
	}

	assertContains(t, buf.String(), "left to right direction", "expected LR direction in PlantUML")
}

// --- String Variant Tests ---

func TestMermaidString_DirectionLR(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "mmd-str-lr")

	out, err := a.Report().WriteMermaidString(auditlog.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteMermaidString error: %v", err)
	}

	assertContains(t, out, "flowchart LR", "expected LR direction in string output")
}

func TestGraphvizString_DirectionLR(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "gv-str-lr")

	out, err := a.Report().WriteGraphvizString(auditlog.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteGraphvizString error: %v", err)
	}

	assertContains(t, out, "rankdir=LR", "expected LR direction in string output")
}

// --- Auditor Delegate Tests ---

func TestAuditor_WriteMermaidWithDirection(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "aud-mmd-lr")

	var buf strings.Builder

	err := a.WriteMermaid(&buf, auditlog.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("Auditor.WriteMermaid error: %v", err)
	}

	assertContains(t, buf.String(), "flowchart LR", "expected LR direction from Auditor")
}

func TestAuditor_WriteGraphvizWithDirection(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "aud-gv-lr")

	var buf strings.Builder

	err := a.WriteGraphviz(&buf, auditlog.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("Auditor.WriteGraphviz error: %v", err)
	}

	assertContains(t, buf.String(), "rankdir=LR", "expected LR direction from Auditor")
}

// --- Export Tests with Direction ---

func TestExportMermaidWithDirection(t *testing.T) {
	t.Parallel()

	a, path := singleSucceedExportPath(t, "export-mmd-lr", "dag.mmd")

	err := a.ExportMermaid(path, auditlog.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("ExportMermaid error: %v", err)
	}
}

// --- Diamond DAG with Direction (ensures direction doesn't break graph topology) ---

func TestDiagram_DiamondDAGWithDirection(t *testing.T) {
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

	report := a.Report()

	// Mermaid LR — ensure all nodes and edges still present.
	var buf strings.Builder

	err := report.WriteMermaid(&buf, auditlog.WithDirection(output.DirectionRight))
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	out := buf.String()
	assertContains(t, out, "flowchart LR", "expected LR direction")
	assertContains(t, out, "root", "expected root node")
	assertContains(t, out, "branch-1", "expected branch-1 node")
	assertContains(t, out, "branch-2", "expected branch-2 node")
	assertContains(t, out, "join", "expected join node")

	// At least 4 edges: root→b1, root→b2, b1→join, b2→join.
	edgeCount := strings.Count(out, "-->")
	if edgeCount < 4 {
		t.Errorf("expected at least 4 edges, got %d", edgeCount)
	}
}
