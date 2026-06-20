package auditlog

import (
	"fmt"

	"github.com/larsartmann/go-output"
)

// stepLabel builds a display label for a step, including retry indicator.
func stepLabel(step StepInfo) string {
	label := step.Name
	if step.AttemptCount > 1 {
		label = fmt.Sprintf("%s (×%d)", label, step.AttemptCount)
	}

	return label
}

// statusStyle maps a StepStatus to an output.GraphStyle for diagram coloring.
// Terminal statuses get fill + font colors; non-terminal statuses get no
// styling (the renderer uses its default node appearance).
func statusStyle(s StepStatus) output.GraphStyle {
	switch s {
	case StepStatusSucceeded:
		return output.GraphStyle{Fill: statusColorSucceeded, FontColor: fontColorLight}
	case StepStatusFailed:
		return output.GraphStyle{Fill: statusColorFailed, FontColor: fontColorLight}
	case StepStatusSkipped:
		return output.GraphStyle{Fill: statusColorSkipped, FontColor: fontColorDim}
	case StepStatusCanceled:
		return output.GraphStyle{Fill: statusColorCanceled, FontColor: fontColorLight}
	default:
		return output.GraphStyle{}
	}
}

// Status fill colors shared across all diagram formats. Font colors are set
// per-status in statusStyle for contrast against the fill.
const (
	statusColorSucceeded = "#2d5a2d" // green
	statusColorFailed    = "#8b2d2d" // red
	statusColorSkipped   = "#4a4a4a" // gray
	statusColorCanceled  = "#5a3d2d" // orange-brown

	fontColorLight = "#fff" // white text on dark fills
	fontColorDim   = "#ccc" // light gray for skipped (lower contrast)
)

// buildGraph converts a WorkflowReport's step DAG into go-output graph nodes
// and edges. Every step becomes a node (colored by status). Each dependency
// becomes an edge pointing from the step to its dependency. Dependencies that
// are not themselves steps in the report are added as plain (uncolored) nodes.
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
			edges = append(edges, *output.NewGraphEdge(step.Name, dep.Name))
		}
	}

	return nodes, edges
}
