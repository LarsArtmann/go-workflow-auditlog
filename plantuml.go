package auditlog

import (
	"io"
	"strings"

	"github.com/larsartmann/go-output/plantuml"
)

// WritePlantUML writes the step dependency DAG as a PlantUML component diagram.
// Nodes are colored by status via inline color specifications.
func (r WorkflowReport) WritePlantUML(writer io.Writer) error {
	renderer := plantuml.NewPlantUMLDiagram()

	return writeGraph(writer, r, "plantuml diagram", renderer)
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
