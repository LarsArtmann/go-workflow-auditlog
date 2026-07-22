package auditlog

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
//	r.WriteGraphviz(w, auditlog.WithDirection(output.DirectionRight))
func (r WorkflowReport) WriteGraphviz(writer io.Writer, opts ...DiagramOption) error {
	renderer := graph.NewDOTRenderer()
	renderer.SetGraphID("workflow")

	cfg := applyDiagramOpts(opts)
	if cfg.hasDirection() {
		renderer.SetDirection(cfg.direction)
	}

	return writeGraph(writer, r, "graphviz diagram", renderer)
}

// WriteGraphvizString returns the Graphviz DOT diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func (r WorkflowReport) WriteGraphvizString(opts ...DiagramOption) (string, error) {
	var buf strings.Builder

	err := r.WriteGraphviz(&buf, opts...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
