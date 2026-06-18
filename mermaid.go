package auditlog

import (
	"fmt"
	"io"
	"strings"
)

// WriteMermaid writes the step dependency DAG as a Mermaid flowchart diagram.
// Nodes are colored by status (green=succeeded, red=failed, gray=skipped).
func (r WorkflowReport) WriteMermaid(writer io.Writer) error {
	return writeDiagram(writer, r, mermaidFormatter{})
}

// WriteMermaidString returns the Mermaid diagram as a string.
func (r WorkflowReport) WriteMermaidString() (string, error) {
	var buf strings.Builder

	err := r.WriteMermaid(&buf)

	return buf.String(), fmt.Errorf("write mermaid: %w", err)
}
