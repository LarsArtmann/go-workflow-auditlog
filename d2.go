package auditlog

import (
	"fmt"
	"io"
	"strings"

	"github.com/larsartmann/go-output"
	"github.com/larsartmann/go-output/d2"
)

// graphNodesToD2 converts go-output GraphNodes to D2Nodes, preserving IDs,
// labels, and fill/font colors from GraphStyle.
func graphNodesToD2(nodes []output.GraphNode) []d2.D2Node {
	result := make([]d2.D2Node, len(nodes))
	for i, node := range nodes {
		result[i] = d2.D2Node{
			ID:    output.NewBrandedID[output.D2NodeIDBrand](node.ID.Get()),
			Label: output.NewBrandedID[output.D2NodeLabelBrand](node.Label.Get()),
			Style: d2.D2NodeStyle{
				Fill:          node.Style.Fill,
				D2StrokeStyle: d2.D2StrokeStyle{FontColor: node.Style.FontColor},
			},
		}
	}

	return result
}

// graphEdgesToD2 converts go-output GraphEdges to D2Edges, preserving IDs.
func graphEdgesToD2(edges []output.GraphEdge) []d2.D2Edge {
	result := make([]d2.D2Edge, len(edges))
	for i, edge := range edges {
		result[i] = d2.D2Edge{
			From: output.NewBrandedID[output.D2NodeIDBrand](edge.From.Get()),
			To:   output.NewBrandedID[output.D2NodeIDBrand](edge.To.Get()),
		}
	}

	return result
}

// WriteD2 writes the step dependency DAG as a D2 diagram.
// Nodes are colored by status (green=succeeded, red=failed, gray=skipped,
// orange=canceled) via inline style attributes.
func (r WorkflowReport) WriteD2(writer io.Writer) error {
	nodes, edges := buildGraph(r)

	diagram := d2.NewD2Diagram()
	diagram.SetTitle("Workflow DAG")

	for _, node := range graphNodesToD2(nodes) {
		diagram.AddNode(node)
	}

	for _, edge := range graphEdgesToD2(edges) {
		diagram.AddEdge(edge)
	}

	out, err := diagram.Render()
	if err != nil {
		return fmt.Errorf("render d2 diagram: %w", err)
	}

	_, err = fmt.Fprintln(writer, out)
	if err != nil {
		return fmt.Errorf("write d2 output: %w", err)
	}

	return nil
}

// WriteD2String returns the D2 diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func (r WorkflowReport) WriteD2String() (string, error) {
	var buf strings.Builder

	err := r.WriteD2(&buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
