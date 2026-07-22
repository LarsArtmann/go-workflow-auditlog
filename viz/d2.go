package auditlog

import (
	"io"
	"strings"

	"github.com/larsartmann/go-output"
	"github.com/larsartmann/go-output/d2"
)

// graphNodesToD2 converts go-output GraphNodes to D2Nodes, preserving IDs,
// labels, and fill/font colors from NodeStyle.
func graphNodesToD2(nodes []output.GraphNode) []d2.Node {
	result := make([]d2.Node, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, d2.Node{
			ID:    output.NewBrandedID[output.D2NodeIDBrand](node.ID.Get()),
			Label: output.NewBrandedID[output.D2NodeLabelBrand](node.Label.Get()),
			Style: d2.NodeStyle{
				Fill:        node.Style.Fill,
				StrokeStyle: d2.StrokeStyle{FontColor: node.Style.FontColor},
			},
		})
	}

	return result
}

// graphEdgesToD2 converts go-output GraphEdges to D2Edges, preserving IDs.
func graphEdgesToD2(edges []output.GraphEdge) []d2.Edge {
	result := make([]d2.Edge, 0, len(edges))
	for _, edge := range edges {
		result = append(result, d2.Edge{
			From: output.NewBrandedID[output.D2NodeIDBrand](edge.From.Get()),
			To:   output.NewBrandedID[output.D2NodeIDBrand](edge.To.Get()),
		})
	}

	return result
}

// d2DiagramTitle returns the title for the D2 diagram, derived from the
// report's WorkflowID. Falls back to "Workflow DAG" when the ID is empty.
func d2DiagramTitle(r WorkflowReport) string {
	if r.WorkflowID != "" {
		return r.WorkflowID
	}

	return "Workflow DAG"
}

// WriteD2 writes the step dependency DAG as a D2 diagram.
// Nodes are colored by status (green=succeeded, red=failed, gray=skipped,
// orange=canceled) via inline style attributes.
//
// The diagram title is derived from the report's WorkflowID so each rendered
// diagram is self-labeling. When WorkflowID is empty the title falls back to
// "Workflow DAG".
//
// Use WithDirection to change the layout direction (default: top-down):
//
//	r.WriteD2(w, auditlog.WithDirection(output.DirectionRight))
func (r WorkflowReport) WriteD2(writer io.Writer, opts ...DiagramOption) error {
	nodes, edges := buildGraph(r)

	diagram := d2.NewDiagram()
	diagram.SetTitle(d2DiagramTitle(r))

	cfg := applyDiagramOpts(opts)
	if cfg.hasDirection() {
		diagram.SetDirection(d2.Direction(cfg.direction))
	}

	for _, node := range graphNodesToD2(nodes) {
		diagram.AddNode(node)
	}

	for _, edge := range graphEdgesToD2(edges) {
		diagram.AddEdge(edge)
	}

	return writeRendered(writer, "d2 diagram", diagram.Render)
}

// WriteD2String returns the D2 diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func (r WorkflowReport) WriteD2String(opts ...DiagramOption) (string, error) {
	var buf strings.Builder

	err := r.WriteD2(&buf, opts...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
