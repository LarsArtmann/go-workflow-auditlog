package viz

import (
	"io"
	"strings"

	"github.com/larsartmann/go-output"
	"github.com/larsartmann/go-output/markup"
	"github.com/larsartmann/go-output/tree"
)

// buildTreeNodes constructs a forest of TreeNodes from the step DAG.
// Root nodes are steps with no dependencies; children are their dependents.
// The result is wrapped in a single root node for the renderer.
func buildTreeNodes(r WorkflowReport) *output.TreeNode {
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

			childNode := output.NewTreeNode(childStep.Name, stepLabel(childStep))
			parent.AddChild(childNode)
			addChildren(childNode, childStep)
		}
	}

	for _, rootStep := range roots {
		rootNode := output.NewTreeNode(rootStep.Name, stepLabel(rootStep))
		forestRoot.AddChild(rootNode)
		addChildren(rootNode, rootStep)
	}

	return forestRoot
}

// WriteTree writes the step dependency DAG as an ASCII tree.
// Nodes are labeled with step name, status, and retry count.
func WriteTree(r WorkflowReport, writer io.Writer) error {
	renderer := tree.NewASCIITreeRenderer()
	renderer.SetRoot(buildTreeNodes(r))

	return writeRendered(writer, "tree", renderer.Render)
}

// WriteTreeString returns the ASCII tree as a string.
// Returns a non-nil error only if tree generation fails.
func WriteTreeString(r WorkflowReport) (string, error) {
	var buf strings.Builder

	err := WriteTree(r, &buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ExportTree writes the ASCII tree to path.
func ExportTree(r WorkflowReport, path string) error {
	return WriteToFile(path, func(w io.Writer) error {
		return WriteTree(r, w)
	})
}

// WriteHTMLTree writes the step dependency DAG as an HTML nested list tree.
// Nodes are labeled with step name, status, and retry count.
func WriteHTMLTree(r WorkflowReport, writer io.Writer) error {
	renderer := markup.NewHTMLTreeRenderer()
	renderer.SetRoot(buildTreeNodes(r))

	return writeRendered(writer, "html tree", renderer.Render)
}

// WriteHTMLTreeString returns the HTML tree as a string.
// Returns a non-nil error only if tree generation fails.
func WriteHTMLTreeString(r WorkflowReport) (string, error) {
	var buf strings.Builder

	err := WriteHTMLTree(r, &buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ExportHTMLTree writes the HTML tree to path.
func ExportHTMLTree(r WorkflowReport, path string) error {
	return WriteToFile(path, func(w io.Writer) error {
		return WriteHTMLTree(r, w)
	})
}
