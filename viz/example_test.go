package viz_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

// exampleStep is used by the Example functions.
type exampleStep struct {
	name string
}

func (s *exampleStep) Do(_ context.Context) error { return nil }
func (s *exampleStep) String() string             { return s.name }

// Example_basicUsage shows the minimal workflow audit setup.
func Example_basicUsage() {
	audit, _ := auditlog.New(auditlog.Config{
		Enabled:    true,
		WorkflowID: "demo",
	})

	step := &exampleStep{name: "fetch"}
	w := &flow.Workflow{}
	w.Add(flow.Step(step))

	audit.Attach(w)
	_ = w.Do(context.Background())
	audit.Snapshot(w)

	report := audit.Report()
	fmt.Printf("Steps: %d, Succeeded: %d\n", report.StepCount, report.SucceededCount)

	// Output:
	// Steps: 1, Succeeded: 1
}

// Example_exportToFile shows how to export the audit report to JSON.
func Example_exportToFile() {
	audit, _ := auditlog.New(auditlog.Config{Enabled: true})

	w := &flow.Workflow{}
	w.Add(flow.Step(&exampleStep{name: "step"}))

	audit.Attach(w)
	_ = w.Do(context.Background())
	audit.Snapshot(w)

	_ = audit.ExportJSON(os.TempDir() + "/audit-example.json")

	fmt.Println("exported")

	// Output:
	// exported
}

// Example_mermaidDiagram shows how to generate a Mermaid DAG visualization.
// The diagram is written to any io.Writer — here we use io.Discard since the
// output is non-deterministic across runs.
func Example_mermaidDiagram() {
	audit, _ := auditlog.New(auditlog.Config{Enabled: true})

	a := &exampleStep{name: "fetch"}
	b := &exampleStep{name: "save"}

	w := &flow.Workflow{}
	w.Add(
		flow.Step(a),
		flow.Step(b).DependsOn(a),
	)

	audit.Attach(w)
	_ = w.Do(context.Background())
	audit.Snapshot(w)

	report := audit.Report()
	_ = viz.WriteMermaid(report, io.Discard)

	// Output:
}

// Example_filtering shows how to filter a report.
func Example_filtering() {
	audit, _ := auditlog.New(auditlog.Config{Enabled: true})

	w := &flow.Workflow{}
	w.Add(
		flow.Step(&exampleStep{name: "ok"}),
		flow.Step(flow.Func("bad", func(_ context.Context) error { return errors.New("fail") })),
	)

	audit.Attach(w)
	_ = w.Do(context.Background())
	audit.Snapshot(w)

	filtered := audit.ReportFiltered(auditlog.WithStepsByStatus(auditlog.StepStatusSucceeded))
	fmt.Printf("Filtered steps: %d\n", filtered.StepCount)

	// Output:
	// Filtered steps: 1
}

// ExampleWriteD2 demonstrates generating a D2 diagram from a report.
func ExampleWriteD2() {
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "a"}},
			{StepRef: auditlog.StepRef{Name: "b"}, Dependencies: []auditlog.StepRef{{Name: "a"}}},
		},
	}

	out, _ := viz.WriteD2String(report)
	fmt.Println(strings.Contains(out, "a"))

	// Output: true
}

// ExampleWriteGraphviz demonstrates generating a Graphviz DOT diagram.
func ExampleWriteGraphviz() {
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "a"}},
			{StepRef: auditlog.StepRef{Name: "b"}, Dependencies: []auditlog.StepRef{{Name: "a"}}},
		},
	}

	out, _ := viz.WriteGraphvizString(report)
	fmt.Println(strings.Contains(out, "digraph"))

	// Output: true
}

// ExampleWritePlantUML demonstrates generating a PlantUML diagram.
func ExampleWritePlantUML() {
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "a"}},
		},
	}

	out, _ := viz.WritePlantUMLString(report)
	fmt.Println(strings.Contains(out, "@startuml"))

	// Output: true
}

// ExampleWriteTree demonstrates generating an ASCII tree view of the step DAG.
func ExampleWriteTree() {
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "fetch"}},
			{StepRef: auditlog.StepRef{Name: "save"}, Dependencies: []auditlog.StepRef{{Name: "fetch"}}},
		},
	}

	out, _ := viz.WriteTreeString(report)
	fmt.Println(strings.Contains(out, "fetch"))

	// Output: true
}

// ExampleWriteHTML demonstrates generating the interactive HTML dashboard.
// The output is a self-contained HTML file with embedded CSS and JavaScript.
func ExampleWriteHTML() {
	report := auditlog.WorkflowReport{
		WorkflowID: "demo",
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "fetch"}, Status: auditlog.StepStatusSucceeded},
		},
	}

	out, _ := viz.WriteHTMLString(report)
	fmt.Println(strings.Contains(out, "<!DOCTYPE html>"))

	// Output: true
}
