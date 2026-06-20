package auditlog

import (
	"fmt"
	"io"
	"strings"

	"github.com/larsartmann/go-output/plantuml"
)

// WritePlantUML writes the step dependency DAG as a PlantUML component diagram.
// Nodes are colored by status via inline color specifications.
func (r WorkflowReport) WritePlantUML(writer io.Writer) error {
	nodes, edges := buildGraph(r)

	renderer := plantuml.NewPlantUMLDiagram()
	renderer.SetNodes(nodes)
	renderer.SetEdges(edges)

	out, err := renderer.Render()
	if err != nil {
		return fmt.Errorf("render plantuml diagram: %w", err)
	}

	_, err = fmt.Fprintln(writer, out)
	if err != nil {
		return fmt.Errorf("write plantuml output: %w", err)
	}

	return nil
}

// WritePlantUMLString returns the PlantUML diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func (r WorkflowReport) WritePlantUMLString() (string, error) {
	var buf strings.Builder

	err := r.WritePlantUML(&buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
