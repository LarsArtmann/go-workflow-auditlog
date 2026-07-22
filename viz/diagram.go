package auditlog

import (
	"fmt"

	"github.com/larsartmann/go-output"
)

// stepLabel builds a display label for a step, including status label and
// retry indicator. Used by all diagram and tree renderers for consistency.
func stepLabel(step StepInfo) string {
	label := step.Name

	statusLabel := step.Status.Label()
	if statusLabel != "" {
		label = fmt.Sprintf("%s [%s]", label, statusLabel)
	}

	if step.AttemptCount > 1 {
		label = fmt.Sprintf("%s (×%d)", label, step.AttemptCount)
	}

	return label
}

// statusStyle maps a StepStatus to an output.NodeStyle for diagram coloring.
// Delegates to StepStatus.Color() so all color definitions live in one place.
func statusStyle(s StepStatus) output.NodeStyle {
	fill, fontColor := s.Color()
	if fill == "" {
		return output.NodeStyle{}
	}

	return output.NodeStyle{Fill: fill, FontColor: fontColor}
}

// buildGraph converts a WorkflowReport's step DAG into go-output graph nodes
// and edges. Every step becomes a node (colored by status). Each dependency
// becomes an edge pointing from the dependency to the step that depends on it
// (dependency → step), so arrows follow execution flow — matching the tree
// export and the convention used by GitHub Actions, Airflow, and Tekton.
// Dependencies that are not themselves steps in the report are added as plain
// (uncolored) nodes.
func buildGraph(report WorkflowReport) ([]output.GraphNode, []output.GraphEdge) {
	seen := make(map[string]struct{})

	var nodes []output.GraphNode

	var edges []output.GraphEdge

	addNode := func(name, label string, status StepStatus) {
		if _, ok := seen[name]; ok {
			return
		}

		seen[name] = struct{}{}

		node := output.NewGraphNode(name, label)
		node.Style = statusStyle(status)
		nodes = append(nodes, *node)
	}

	for _, step := range report.Steps {
		addNode(step.Name, stepLabel(step), step.Status)
	}

	for _, step := range report.Steps {
		for _, dep := range step.Dependencies {
			addNode(dep.Name, dep.Name, StepStatusPending)
			// Edge points dependency → step (forward execution flow).
			edges = append(edges, *output.NewGraphEdge(dep.Name, step.Name))
		}
	}

	return nodes, edges
}
