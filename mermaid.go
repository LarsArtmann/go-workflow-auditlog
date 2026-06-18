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
// Returns a non-nil error only if diagram generation fails.
func (r WorkflowReport) WriteMermaidString() (string, error) {
	var buf strings.Builder

	err := r.WriteMermaid(&buf)
	if err != nil {
		return "", fmt.Errorf("write mermaid: %w", err)
	}

	return buf.String(), nil
}
