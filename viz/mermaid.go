package viz

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
//	viz.WriteMermaid(w, report, viz.WithDirection(output.DirectionRight))
func WriteMermaid(r WorkflowReport, writer io.Writer, opts ...DiagramOption) error {
	renderer := graph.NewMermaidRenderer()
	renderer.SetCodeFence(false)

	cfg := applyDiagramOpts(opts)

	transform := func(out string) string {
		if cfg.hasDirection() {
			return strings.Replace(out, "flowchart TD", "flowchart "+mermaidDirection(cfg.direction), 1)
		}

		return out
	}

	return writeGraph(writer, r, "mermaid diagram", renderer, transform)
}

// WriteMermaidString returns the Mermaid diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func WriteMermaidString(r WorkflowReport, opts ...DiagramOption) (string, error) {
	var buf strings.Builder

	err := WriteMermaid(r, &buf, opts...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ExportMermaid writes the Mermaid diagram to path.
func ExportMermaid(r WorkflowReport, path string, opts ...DiagramOption) error {
	return WriteToFile(path, func(w io.Writer) error {
		return WriteMermaid(r, w, opts...)
	})
}
