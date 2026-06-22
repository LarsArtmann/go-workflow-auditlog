package auditlog

import (
	"fmt"
	"io"
	"strings"

	"github.com/larsartmann/go-output/graph"
)

// WriteGraphviz writes the step dependency DAG as a Graphviz DOT digraph.
// Nodes are colored by status (green=succeeded, red=failed, gray=skipped,
// orange=canceled) via fillcolor attributes.
// The output is valid DOT, consumable by `dot -Tsvg` or any Graphviz renderer.
func (r WorkflowReport) WriteGraphviz(writer io.Writer) error {
	nodes, edges := buildGraph(r)

	renderer := graph.NewDOTRenderer()
	renderer.SetGraphID("workflow")
	renderer.SetNodes(nodes)
	renderer.SetEdges(edges)

	out, err := renderer.Render()
	if err != nil {
		return fmt.Errorf("%w: render graphviz diagram: %w", ErrRenderFailed, err)
	}

	_, err = fmt.Fprintln(writer, out)
	if err != nil {
		return fmt.Errorf("%w: write graphviz output: %w", ErrExportWriteFailed, err)
	}

	return nil
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
