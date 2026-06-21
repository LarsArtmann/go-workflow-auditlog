package auditlog

import (
	"fmt"
	"io"
	"strings"

	"github.com/larsartmann/go-output"
	"github.com/larsartmann/go-output/d2"
)

// statusD2Style maps a StepStatus to a D2NodeStyle for diagram coloring.
// Delegates to StepStatus.Color() so all color definitions live in one place.
func statusD2Style(s StepStatus) d2.D2NodeStyle {
	fill, fontColor := s.Color()
	if fill == "" {
		return d2.D2NodeStyle{}
	}

	return d2.D2NodeStyle{
		Fill:          fill,
		D2StrokeStyle: d2.D2StrokeStyle{FontColor: fontColor},
	}
}

// buildD2Graph converts a WorkflowReport's step DAG into D2 nodes and edges.
// Every step becomes a node (colored by status). Each dependency becomes an
// edge pointing from the step to its dependency.
func buildD2Graph(report WorkflowReport) ([]d2.D2Node, []d2.D2Edge) {
	seen := make(map[string]struct{})

	var (
		nodes []d2.D2Node
		edges []d2.D2Edge
	)

	addNode := func(name, label string, status StepStatus) {
		if _, ok := seen[name]; ok {
			return
		}

		seen[name] = struct{}{}

		node := d2.D2Node{
			ID:    output.NewBrandedID[output.D2NodeIDBrand](name),
			Label: output.NewBrandedID[output.D2NodeLabelBrand](label),
			Style: statusD2Style(status),
		}
		nodes = append(nodes, node)
	}

	for _, step := range report.Steps {
		addNode(step.Name, stepLabel(step), step.Status)
	}

	for _, step := range report.Steps {
		for _, dep := range step.Dependencies {
			addNode(dep.Name, dep.Name, StepStatusPending)
			edges = append(edges, d2.D2Edge{
				From: output.NewBrandedID[output.D2NodeIDBrand](step.Name),
				To:   output.NewBrandedID[output.D2NodeIDBrand](dep.Name),
			})
		}
	}

	return nodes, edges
}

// WriteD2 writes the step dependency DAG as a D2 diagram.
// Nodes are colored by status (green=succeeded, red=failed, gray=skipped,
// orange=canceled) via inline style attributes.
func (r WorkflowReport) WriteD2(writer io.Writer) error {
	nodes, edges := buildD2Graph(r)

	diagram := d2.NewD2Diagram()
	diagram.SetTitle("Workflow DAG")

	for _, node := range nodes {
		diagram.AddNode(node)
	}

	for _, edge := range edges {
		diagram.AddEdge(edge)
	}

	out, err := diagram.Render()
	if err != nil {
		return fmt.Errorf("render d2 diagram: %w", err)
	}

	_, err = fmt.Fprintln(writer, out)
	if err != nil {
		return fmt.Errorf("write d2 output: %w", err)
	}

	return nil
}

// WriteD2String returns the D2 diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func (r WorkflowReport) WriteD2String() (string, error) {
	var buf strings.Builder

	err := r.WriteD2(&buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
