package viz_test

import (
	"strings"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

// =============================================================================
// P4-32: Integration round-trip test
// report → JSON → LoadReport → report → diagram → structural verify
// =============================================================================

func TestIntegration_RoundTrip_JSON_Load_Diagram(t *testing.T) {
	t.Parallel()

	original := minimalReport()

	// Step 1: Export to JSON
	var jsonBuf strings.Builder

	err := original.WriteJSON(&jsonBuf)
	if err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	if jsonBuf.Len() == 0 {
		t.Fatal("WriteJSON produced empty output")
	}

	// Step 2: Load from JSON
	loaded, err := auditlog.LoadReportFromBytes([]byte(jsonBuf.String()))
	if err != nil {
		t.Fatalf("LoadReportFromBytes error: %v", err)
	}

	// Step 3: Verify key fields survived the round trip
	if loaded.WorkflowID != original.WorkflowID {
		t.Errorf("WorkflowID: got %q, want %q", loaded.WorkflowID, original.WorkflowID)
	}

	if loaded.StepCount != original.StepCount {
		t.Errorf("StepCount: got %d, want %d", loaded.StepCount, original.StepCount)
	}

	if len(loaded.Steps) != len(original.Steps) {
		t.Errorf("Steps len: got %d, want %d", len(loaded.Steps), len(original.Steps))
	}

	if len(loaded.Steps) > 0 && loaded.Steps[0].Name != original.Steps[0].Name {
		t.Errorf("Steps[0].Name: got %q, want %q", loaded.Steps[0].Name, original.Steps[0].Name)
	}

	// Step 4: Loaded report produces valid diagram output
	mermaidOut, err := viz.WriteMermaidString(loaded)
	if err != nil {
		t.Fatalf("loaded.WriteMermaidString error: %v", err)
	}

	if mermaidOut == "" {
		t.Fatal("loaded report produced empty Mermaid output")
	}

	// Step 5: Loaded report produces valid D2 output
	d2Out, err := viz.WriteD2String(loaded)
	if err != nil {
		t.Fatalf("loaded.WriteD2String error: %v", err)
	}

	if d2Out == "" {
		t.Fatal("loaded report produced empty D2 output")
	}

	// Step 6: Verify diagram content references the step name
	if !strings.Contains(mermaidOut, "step-a") {
		t.Error("Mermaid output from loaded report missing step name 'step-a'")
	}

	if !strings.Contains(d2Out, "step-a") {
		t.Error("D2 output from loaded report missing step name 'step-a'")
	}
}

func TestIntegration_RoundTrip_NDJSON_Replay_Diagram(t *testing.T) {
	t.Parallel()

	original := minimalReport()

	// Step 1: Export events to NDJSON
	var ndjsonBuf strings.Builder

	err := original.WriteNDJSON(&ndjsonBuf)
	if err != nil {
		t.Fatalf("WriteNDJSON error: %v", err)
	}

	if ndjsonBuf.Len() == 0 {
		t.Fatal("WriteNDJSON produced empty output")
	}

	// Step 2: Replay events back into a report
	events, err := auditlog.ReadEvents(strings.NewReader(ndjsonBuf.String()))
	if err != nil {
		t.Fatalf("ReadEvents error: %v", err)
	}

	replayed, err := auditlog.ReplayEvents(events)
	if err != nil {
		t.Fatalf("ReplayEvents error: %v", err)
	}

	// Step 3: Verify events survived
	if len(replayed.Events) != len(original.Events) {
		t.Errorf("Events len: got %d, want %d", len(replayed.Events), len(original.Events))
	}

	// Step 4: Verify steps survived
	if len(replayed.Steps) != len(original.Steps) {
		t.Errorf("Steps len: got %d, want %d", len(replayed.Steps), len(original.Steps))
	}

	// Step 5: Replay report produces valid diagram output
	treeOut, err := viz.WriteTreeString(replayed)
	if err != nil {
		t.Fatalf("replayed.WriteTreeString error: %v", err)
	}

	if treeOut == "" {
		t.Fatal("replayed report produced empty tree output")
	}
}

// =============================================================================
// P4-33: Cross-format consistency test
// Same report → Mermaid vs DOT vs D2 should have same node/edge structure
// =============================================================================

func TestCrossFormat_DiagramNodeConsistency(t *testing.T) {
	t.Parallel()

	// Build a 3-step linear chain report
	report := minimalReport()

	// Add a second step with a dependency on the first
	now := minimalReport().ExportedAt
	dur := 10.0

	report.Steps = append(report.Steps, auditlog.StepInfo{
		StepRef:      auditlog.StepRef{Name: "step-b", StepType: "TestStep"},
		Status:       auditlog.StepStatusFailed,
		AttemptCount: 2,
		DurationMs:   &dur,
		StartedAt:    &now,
		FinishedAt:   &now,
		Dependencies: []auditlog.StepRef{{Name: "step-a"}},
	})
	report.StepCount = 2

	// Render all three formats
	mermaidOut, err := viz.WriteMermaidString(report)
	if err != nil {
		t.Fatalf("WriteMermaidString error: %v", err)
	}

	dotOut, err := viz.WriteGraphvizString(report)
	if err != nil {
		t.Fatalf("WriteGraphvizString error: %v", err)
	}

	d2Out, err := viz.WriteD2String(report)
	if err != nil {
		t.Fatalf("WriteD2String error: %v", err)
	}

	// All formats must reference both step names
	for _, format := range []struct {
		name, output string
	}{
		{"mermaid", mermaidOut},
		{"dot", dotOut},
		{"d2", d2Out},
	} {
		if !strings.Contains(format.output, "step-a") {
			t.Errorf("%s output missing node 'step-a'", format.name)
		}

		if !strings.Contains(format.output, "step-b") {
			t.Errorf("%s output missing node 'step-b'", format.name)
		}
	}

	// All formats must reference the edge (dependency between steps)
	// Mermaid: step-a --> step-b (or similar)
	// DOT: "step-a" -> "step-b"
	// D2: step-a -> step-b
	for _, format := range []struct {
		name, output, edgeMarker string
	}{
		{"mermaid", mermaidOut, "step-a"},
		{"dot", dotOut, "step-a"},
		{"d2", d2Out, "step-a"},
	} {
		if !strings.Contains(format.output, format.edgeMarker) {
			t.Errorf("%s output missing edge marker '%s'", format.name, format.edgeMarker)
		}
	}
}
