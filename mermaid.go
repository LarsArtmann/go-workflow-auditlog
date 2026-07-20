package auditlog

import (
	"io"
	"strings"

	"github.com/larsartmann/go-output/graph"
)

// WriteMermaid writes the step dependency DAG as a Mermaid flowchart diagram.
// Nodes are colored by status (green=succeeded, red=failed, gray=skipped,
// orange=canceled) via per-node style directives.
//
// The output is raw flowchart syntax (no ```mermaid code fence) so it can be
// written to .mmd files or embedded directly.
func (r WorkflowReport) WriteMermaid(writer io.Writer) error {
	renderer := graph.NewMermaidRenderer()
	renderer.SetCodeFence(false)

	return writeGraph(writer, r, "mermaid diagram", renderer)
}

// WriteMermaidString returns the Mermaid diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func (r WorkflowReport) WriteMermaidString() (string, error) {
	var buf strings.Builder

	err := r.WriteMermaid(&buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
