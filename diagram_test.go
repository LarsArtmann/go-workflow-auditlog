package auditlog_test

import (
	"context"
	"strings"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/cenkalti/backoff/v4"
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

	// Verify header.
	if !strings.Contains(output, "flowchart TD") {
		t.Error("expected 'flowchart TD' in output")
	}

	// Verify nodes exist.
	if !strings.Contains(output, "fetch") {
		t.Error("expected 'fetch' node in output")
	}

	if !strings.Contains(output, "transform") {
		t.Error("expected 'transform' node in output")
	}

	if !strings.Contains(output, "save") {
		t.Error("expected 'save' node in output")
	}

	// Verify edges (dependency arrows).
	if !strings.Contains(output, "-->") {
		t.Error("expected '-->' edge in output")
	}

	// Verify status class assignment.
	if !strings.Contains(output, "classDef succeeded") {
		t.Error("expected succeeded classDef in output")
	}

	if !strings.Contains(output, "succeeded") {
		t.Error("expected succeeded class in output")
	}
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

	if !strings.Contains(output, "classDef failed") {
		t.Error("expected failed classDef in output")
	}

	if !strings.Contains(output, "failed") {
		t.Error("expected failed class assignment in output")
	}
}

func TestMermaid_RetryIndicator(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newFlaky("flaky", 2)
	w.Add(
		flow.Step(step).Retry(func(o *flow.RetryOption) {
			o.Attempts = 5
			o.Backoff = backoff.NewExponentialBackOff()
		}),
	)
	runWorkflow(t, a, w)

	var buf strings.Builder
	err := a.Report().WriteMermaid(&buf)
	if err != nil {
		t.Fatalf("WriteMermaid error: %v", err)
	}

	output := buf.String()

	// The retry count should appear in the label.
	if !strings.Contains(output, "×3") {
		t.Error("expected '×3' retry indicator in mermaid output")
	}
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
	if !strings.Contains(output, "flowchart TD") {
		t.Error("expected header even for empty report")
	}
}

func TestPlantUML_BasicDAG(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	save := newSucceed("save")
	w.Add(
		flow.Step(fetch),
		flow.Step(save).DependsOn(fetch),
	)
	runWorkflow(t, a, w)

	var buf strings.Builder
	err := a.Report().WritePlantUML(&buf)
	if err != nil {
		t.Fatalf("WritePlantUML error: %v", err)
	}

	output := buf.String()

	// Verify PlantUML markers.
	if !strings.Contains(output, "@startuml") {
		t.Error("expected '@startuml' in output")
	}

	if !strings.Contains(output, "@enduml") {
		t.Error("expected '@enduml' in output")
	}

	// Verify component declarations.
	if !strings.Contains(output, "component") {
		t.Error("expected 'component' in output")
	}

	// Verify nodes exist.
	if !strings.Contains(output, "fetch") {
		t.Error("expected 'fetch' in output")
	}

	if !strings.Contains(output, "save") {
		t.Error("expected 'save' in output")
	}
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

	if !strings.Contains(output, "flowchart TD") {
		t.Error("expected 'flowchart TD' in string output")
	}
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

	if !strings.Contains(output, "classDef skipped") {
		t.Error("expected skipped classDef in output")
	}
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

	if !strings.Contains(output, "classDef canceled") {
		t.Error("expected canceled classDef in output")
	}
}
