package auditlog

import (
	"io"
	"strings"

	"github.com/larsartmann/go-output/plantuml"
)

// WritePlantUML writes the step dependency DAG as a PlantUML component diagram.
// Nodes are colored by status via inline color specifications.
//
// Use WithDirection to change the layout direction. PlantUML supports
// top-to-bottom (default) and left-to-right (DirectionRight/DirectionLeft):
//
//	r.WritePlantUML(w, auditlog.WithDirection(output.DirectionRight))
func (r WorkflowReport) WritePlantUML(writer io.Writer, opts ...DiagramOption) error {
	renderer := plantuml.NewPlantUMLDiagram()

	nodes, edges := buildGraph(r)
	renderer.SetNodes(nodes)
	renderer.SetEdges(edges)

	cfg := applyDiagramOpts(opts)

	return writeRenderedTransformed(writer, "plantuml diagram", renderer.Render, func(out string) string {
		if cfg.hasDirection() {
			return applyPlantumlDirection(out, plantumlDirectionCommand(cfg.direction))
		}

		return out
	})
}

// WritePlantUMLString returns the PlantUML diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func (r WorkflowReport) WritePlantUMLString(opts ...DiagramOption) (string, error) {
	var buf strings.Builder

	err := r.WritePlantUML(&buf, opts...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
