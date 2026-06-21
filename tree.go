package auditlog

import (
	"fmt"
	"io"
	"strings"

	"github.com/larsartmann/go-output"
	"github.com/larsartmann/go-output/markup"
	"github.com/larsartmann/go-output/tree"
)

// treeStepLabel builds a display label for a step in tree output, including
// status and retry indicator.
func treeStepLabel(step StepInfo) string {
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

// buildTreeNodes constructs a forest of TreeNodes from the step DAG.
// Root nodes are steps with no dependencies; children are their dependents.
// The result is wrapped in a single root node for the renderer.
func (r WorkflowReport) buildTreeNodes() *output.TreeNode {
	forestRoot := output.NewTreeNode("workflow", "Workflow")

	if len(r.Steps) == 0 {
		return forestRoot
	}

	// Build lookup map from step name to StepInfo.
	byName := make(map[string]StepInfo, len(r.Steps))
	for _, step := range r.Steps {
		byName[step.Name] = step
	}

	// Find root steps: those with no dependencies.
	var roots []StepInfo

	for _, step := range r.Steps {
		if len(step.Dependencies) == 0 {
			roots = append(roots, step)
		}
	}

	// If every step has dependencies (e.g. a cyclic workflow, which shouldn't
	// happen), fall back to using the first step as root.
	if len(roots) == 0 && len(r.Steps) > 0 {
		roots = append(roots, r.Steps[0])
	}

	// Track visited to avoid infinite recursion on unexpected cycles.
	visited := make(map[string]struct{})

	var addChildren func(parent *output.TreeNode, step StepInfo)

	addChildren = func(parent *output.TreeNode, step StepInfo) {
		if _, ok := visited[step.Name]; ok {
			return
		}

		visited[step.Name] = struct{}{}

		for _, depRef := range step.Dependents {
			childStep, ok := byName[depRef.Name]
			if !ok {
				continue
			}

			childNode := output.NewTreeNode(childStep.Name, treeStepLabel(childStep))
			parent.AddChild(childNode)
			addChildren(childNode, childStep)
		}
	}

	for _, rootStep := range roots {
		rootNode := output.NewTreeNode(rootStep.Name, treeStepLabel(rootStep))
		forestRoot.AddChild(rootNode)
		addChildren(rootNode, rootStep)
	}

	return forestRoot
}

// WriteTree writes the step dependency DAG as an ASCII tree.
// Nodes are labeled with step name, status, and retry count.
func (r WorkflowReport) WriteTree(writer io.Writer) error {
	root := r.buildTreeNodes()

	renderer := tree.NewASCIITreeRenderer()
	renderer.SetRoot(root)

	out, err := renderer.Render()
	if err != nil {
		return fmt.Errorf("render tree: %w", err)
	}

	_, err = fmt.Fprintln(writer, out)
	if err != nil {
		return fmt.Errorf("write tree output: %w", err)
	}

	return nil
}

// WriteTreeString returns the ASCII tree as a string.
// Returns a non-nil error only if tree generation fails.
func (r WorkflowReport) WriteTreeString() (string, error) {
	var buf strings.Builder

	err := r.WriteTree(&buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// WriteHTMLTree writes the step dependency DAG as an HTML nested list tree.
// Nodes are labeled with step name, status, and retry count.
func (r WorkflowReport) WriteHTMLTree(writer io.Writer) error {
	root := r.buildTreeNodes()

	renderer := markup.NewHTMLTreeRenderer()
	renderer.SetRoot(root)

	out, err := renderer.Render()
	if err != nil {
		return fmt.Errorf("render html tree: %w", err)
	}

	_, err = fmt.Fprintln(writer, out)
	if err != nil {
		return fmt.Errorf("write html tree output: %w", err)
	}

	return nil
}

// WriteHTMLTreeString returns the HTML tree as a string.
// Returns a non-nil error only if tree generation fails.
func (r WorkflowReport) WriteHTMLTreeString() (string, error) {
	var buf strings.Builder

	err := r.WriteHTMLTree(&buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
