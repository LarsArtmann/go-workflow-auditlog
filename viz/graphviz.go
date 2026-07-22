package viz

import (
	"io"
	"strings"

	"github.com/larsartmann/go-output/graph"
)

// WriteGraphviz writes the step dependency DAG as a Graphviz DOT digraph.
// Nodes are colored by status (green=succeeded, red=failed, gray=skipped,
// orange=canceled) via fillcolor attributes.
// The output is valid DOT, consumable by `dot -Tsvg` or any Graphviz renderer.
//
// Use WithDirection to change the layout direction (default: top-down):
//
//	viz.WriteGraphviz(w, report, viz.WithDirection(output.DirectionRight))
func WriteGraphviz(r WorkflowReport, writer io.Writer, opts ...DiagramOption) error {
	renderer := graph.NewDOTRenderer()
	renderer.SetGraphID("workflow")

	cfg := applyDiagramOpts(opts)
	if cfg.hasDirection() {
		renderer.SetDirection(cfg.direction)
	}

	return writeGraph(writer, r, "graphviz diagram", renderer, nil)
}

// WriteGraphvizString returns the Graphviz DOT diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func WriteGraphvizString(r WorkflowReport, opts ...DiagramOption) (string, error) {
	var buf strings.Builder

	err := WriteGraphviz(r, &buf, opts...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ExportGraphviz writes the Graphviz DOT diagram to path.
func ExportGraphviz(r WorkflowReport, path string, opts ...DiagramOption) error {
	return WriteToFile(path, func(w io.Writer) error {
		return WriteGraphviz(r, w, opts...)
	})
}
