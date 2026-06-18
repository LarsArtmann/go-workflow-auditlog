package auditlog_test

import (
	"context"
	"errors"
	"fmt"
	"os"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
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

	_ = audit.ExportToFile(os.TempDir() + "/audit-example.json")

	fmt.Println("exported")

	// Output:
	// exported
}

// Example_mermaidDiagram shows how to generate a Mermaid DAG visualization.
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
	_ = report.WriteMermaid(os.Stdout)

	// Output will contain "flowchart TD" and the step names.
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
