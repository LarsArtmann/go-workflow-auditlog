package viz

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
//	viz.WritePlantUML(w, report, viz.WithDirection(output.DirectionRight))
func WritePlantUML(r WorkflowReport, writer io.Writer, opts ...DiagramOption) error {
	renderer := plantuml.NewPlantUMLDiagram()

	cfg := applyDiagramOpts(opts)

	transform := func(out string) string {
		if cfg.hasDirection() {
			return applyPlantumlDirection(out, plantumlDirectionCommand(cfg.direction))
		}

		return out
	}

	return writeGraph(writer, r, "plantuml diagram", renderer, transform)
}

// WritePlantUMLString returns the PlantUML diagram as a string.
// Returns a non-nil error only if diagram generation fails.
func WritePlantUMLString(r WorkflowReport, opts ...DiagramOption) (string, error) {
	var buf strings.Builder

	err := WritePlantUML(r, &buf, opts...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ExportPlantUML writes the PlantUML diagram to path.
func ExportPlantUML(r WorkflowReport, path string, opts ...DiagramOption) error {
	return WriteToFile(path, func(w io.Writer) error {
		return WritePlantUML(r, w, opts...)
	})
}
