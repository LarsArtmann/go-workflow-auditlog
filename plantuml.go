package auditlog

import (
	"fmt"
	"io"
	"strings"
)

// WritePlantUML writes the step dependency DAG as a PlantUML component diagram.
func (r WorkflowReport) WritePlantUML(writer io.Writer) error {
	return writeDiagram(writer, r, plantumlFormatter{})
}

// WritePlantUMLString returns the PlantUML diagram as a string.
func (r WorkflowReport) WritePlantUMLString() (string, error) {
	var buf strings.Builder

	err := r.WritePlantUML(&buf)

	return buf.String(), fmt.Errorf("write plantuml: %w", err)
}
