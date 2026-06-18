package auditlog

import (
	"io"
	"strings"
)

// WriteGraphviz writes the step dependency DAG as a Graphviz DOT digraph.
// Nodes are colored by status (green=succeeded, red=failed, gray=skipped).
// The output is valid DOT, consumable by `dot -Tsvg` or any Graphviz renderer.
func (r WorkflowReport) WriteGraphviz(writer io.Writer) error {
	return writeDiagram(writer, r, dotFormatter{})
}

// WriteGraphvizString returns the Graphviz DOT diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func (r WorkflowReport) WriteGraphvizString() (string, error) {
	var buf strings.Builder

	err := r.WriteGraphviz(&buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
