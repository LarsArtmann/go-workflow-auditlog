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
//
// Use WithDirection to change the layout direction (default: TD):
//
//	r.WriteMermaid(w, auditlog.WithDirection(output.DirectionRight))
func (r WorkflowReport) WriteMermaid(writer io.Writer, opts ...DiagramOption) error {
	renderer := graph.NewMermaidRenderer()
	renderer.SetCodeFence(false)

	nodes, edges := buildGraph(r)
	renderer.SetNodes(nodes)
	renderer.SetEdges(edges)

	cfg := applyDiagramOpts(opts)

	return writeRenderedTransformed(writer, "mermaid diagram", renderer.Render, func(out string) string {
		if cfg.hasDirection() {
			return strings.Replace(out, "flowchart TD", "flowchart "+mermaidDirection(cfg.direction), 1)
		}

		return out
	})
}

// WriteMermaidString returns the Mermaid diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func (r WorkflowReport) WriteMermaidString(opts ...DiagramOption) (string, error) {
	var buf strings.Builder

	err := r.WriteMermaid(&buf, opts...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
