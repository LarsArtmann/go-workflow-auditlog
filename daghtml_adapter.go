package auditlog

import (
	"fmt"

	"github.com/larsartmann/go-output/daghtml"
)

// stepStatusDAGColor returns the CSS color token used by the daghtml
// visualization for the given StepStatus. Mirrors the status color mapping
// in the former dashboard.js renderGraph() function.
func stepStatusDAGColor(status StepStatus) string {
	switch status {
	case StepStatusSucceeded:
		return "var(--success)"
	case StepStatusFailed:
		return "var(--error)"
	case StepStatusRunning:
		return "var(--warning)"
	case StepStatusPending:
		return "var(--text-muted)"
	case StepStatusCanceled:
		return "var(--transient)"
	case StepStatusSkipped:
		return "var(--text-dim)"
	default:
		return "var(--accent)"
	}
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

		color := stepStatusDAGColor(step.Status)

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
