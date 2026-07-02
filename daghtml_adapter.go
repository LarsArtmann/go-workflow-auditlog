package auditlog

import (
	"fmt"

	"github.com/larsartmann/go-output/daghtml"
)

// stepStatusDAGColor maps a StepStatus to the CSS color token used by the
// daghtml visualization. Mirrors the status color mapping in the former
// dashboard.js renderGraph() function.
var stepStatusDAGColor = map[StepStatus]string{
	StepStatusSucceeded: "var(--success)",
	StepStatusFailed:    "var(--error)",
	StepStatusRunning:   "var(--warning)",
	StepStatusPending:   "var(--text-muted)",
	StepStatusCanceled:  "var(--transient)",
	StepStatusSkipped:   "var(--text-dim)",
}

// buildDAGHTML converts a WorkflowReport into a daghtml.DAG suitable for the
// interactive HTML graph renderer. Each step becomes a node; each dependency
// edge becomes a directed edge from dependency → step.
func buildDAGHTML(report WorkflowReport) daghtml.DAG {
	dag := daghtml.DAG{
		Nodes: make([]daghtml.Node, 0, len(report.Steps)),
		Edges: make([]daghtml.Edge, 0),
	}

	for _, step := range report.Steps {
		icon := stepStatusMeta[step.Status].Icon
		label := step.Name
		if icon != "" {
			label = icon + " " + label
		}

		color := stepStatusDAGColor[step.Status]
		if color == "" {
			color = "var(--accent)"
		}

		dag.Nodes = append(dag.Nodes, daghtml.Node{
			ID:      step.Name,
			Label:   label,
			Color:   color,
			Tooltip: buildStepTooltip(step),
			Error:   step.Status == StepStatusFailed || step.Status == StepStatusCanceled,
		})
	}

	for _, step := range report.Steps {
		for _, dep := range step.Dependencies {
			dag.Edges = append(dag.Edges, daghtml.Edge{
				From: dep.Name,
				To:   step.Name,
			})
		}
	}

	return dag
}

func buildStepTooltip(step StepInfo) string {
	tip := step.Name
	if step.StepType != "" {
		tip += " | type: " + step.StepType
	}
	tip += " | status: " + string(step.Status)
	tip += fmt.Sprintf(" | attempts: %d", step.AttemptCount)
	if step.DurationMs != nil {
		tip += fmt.Sprintf(" | duration: %.1fms", *step.DurationMs)
	}
	if step.Error != nil {
		tip += " | error: " + *step.Error
	}
	if step.HasRetry {
		tip += " | retry: yes"
	}
	if step.HasTimeout {
		tip += " | timeout: yes"
	}
	return tip
}
