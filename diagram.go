package auditlog

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// diagramFormatter turns a collected DAG into a specific text format.
type diagramFormatter interface {
	Header() string
	Footer() string
	NodeID(name string) string
	NodeDecl(id, label string) string
	EdgeDecl(fromID, toID string) string
	ClassAssign(id, className string) string
}

// diagramEntry is a single line in the generated diagram, paired with its
// sort key so declarations and edges can be deduplicated and ordered.
type diagramEntry struct {
	line string
	key  string
}

// diagramAvgLineBytes is the pre-allocation estimate for each diagram line.
const diagramAvgLineBytes = 64

// writeDiagram writes a dependency graph using the supplied formatter.
// It deduplicates nodes and edges, sorts output for deterministic reports,
// and assigns status-based CSS classes for visual styling.
func writeDiagram(writer io.Writer, report WorkflowReport, formatter diagramFormatter) error {
	seen := make(map[string]struct{})

	var entries []diagramEntry

	add := func(key, line string) {
		if _, ok := seen[key]; ok {
			return
		}

		seen[key] = struct{}{}
		entries = append(entries, diagramEntry{line: line, key: key})
	}

	for _, step := range report.Steps {
		fromID := formatter.NodeID(step.Name)
		add(fromID, formatter.NodeDecl(fromID, stepLabel(step)))

		for _, dep := range step.Dependencies {
			toID := formatter.NodeID(dep.Name)
			add(toID, formatter.NodeDecl(toID, dep.Name))
			add(fromID+"->"+toID, formatter.EdgeDecl(fromID, toID))
		}

		// Assign status-based class for visual styling.
		if class := statusClass(step.Status); class != "" {
			add("class:"+fromID, formatter.ClassAssign(fromID, class))
		}
	}

	slices.SortFunc(entries, func(a, b diagramEntry) int {
		return strings.Compare(a.key, b.key)
	})

	var builder strings.Builder
	builder.Grow(len(entries) * diagramAvgLineBytes)

	builder.WriteString(formatter.Header())
	builder.WriteByte('\n')

	for _, entry := range entries {
		builder.WriteString("    ")
		builder.WriteString(entry.line)
		builder.WriteByte('\n')
	}

	if footer := formatter.Footer(); footer != "" {
		builder.WriteString(footer)
		builder.WriteByte('\n')
	}

	_, err := writer.Write([]byte(builder.String()))
	if err != nil {
		return fmt.Errorf("write diagram: %w", err)
	}

	return nil
}

// stepLabel builds a display label for a step, including retry indicator.
func stepLabel(step StepInfo) string {
	label := step.Name
	if step.AttemptCount > 1 {
		label = fmt.Sprintf("%s (×%d)", label, step.AttemptCount)
	}

	return label
}

// statusClass maps a StepStatus to a CSS class name for diagram styling.
func statusClass(s StepStatus) string {
	switch s {
	case StepStatusSucceeded:
		return "succeeded"
	case StepStatusFailed:
		return "failed"
	case StepStatusSkipped:
		return "skipped"
	case StepStatusCanceled:
		return "canceled"
	default:
		return ""
	}
}

// diagramIDReplacer collapses characters that are invalid in Mermaid/PlantUML
// node identifiers into underscores.
//
//nolint:gochecknoglobals // Reusable strings.Replacer, safe to share.
var diagramIDReplacer = strings.NewReplacer(
	"-", "_",
	" ", "_",
	"/", "_",
	".", "_",
	"*", "_",
	"[", "_",
	"]", "_",
	"(", "_",
	")", "_",
)

// sanitizeDiagramID builds a valid node identifier from a step name.
// Returns "node" if the result would be empty.
func sanitizeDiagramID(name string) string {
	raw := diagramIDReplacer.Replace(name)

	var b strings.Builder
	b.Grow(len(raw))

	for _, r := range raw {
		if isDiagramIdentRune(r) {
			b.WriteRune(r)
		}
	}

	if b.Len() == 0 {
		return "node"
	}

	return b.String()
}

// isDiagramIdentRune reports whether r is valid in a node identifier.
func isDiagramIdentRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}

// mermaidLabelReplacer escapes characters that break Mermaid node labels.
//
//nolint:gochecknoglobals // Reusable strings.Replacer, safe to share.
var mermaidLabelReplacer = strings.NewReplacer(
	`"`, "'",
	"[", "(",
	"]", ")",
	"{", "(",
	"}", ")",
	"\n", "<br>",
)

func mermaidLabel(label string) string {
	return mermaidLabelReplacer.Replace(label)
}

func plantumlLabel(label string) string {
	return strings.ReplaceAll(label, `"`, "'")
}

// --- Mermaid formatter ---

type mermaidFormatter struct{}

func (mermaidFormatter) Header() string {
	return `%%{init: {'theme':'base', 'themeVariables': {'primaryColor':'#e8a838', 'primaryTextColor':'#14110d', 'primaryBorderColor':'#4a4030', 'lineColor':'#9a8d78', 'fontSize':'14px'}}}%%
flowchart TD
    classDef succeeded fill:#2d5a2d,color:#fff
    classDef failed fill:#8b2d2d,color:#fff
    classDef skipped fill:#4a4a4a,color:#ccc
    classDef canceled fill:#5a3d2d,color:#fff`
}

func (mermaidFormatter) Footer() string { return "" }

func (mermaidFormatter) NodeID(name string) string {
	return sanitizeDiagramID(name)
}

func (mermaidFormatter) NodeDecl(id, label string) string {
	return fmt.Sprintf("%s[\"%s\"]", id, mermaidLabel(label))
}

func (mermaidFormatter) EdgeDecl(fromID, toID string) string {
	return fmt.Sprintf("%s --> %s", fromID, toID)
}

func (mermaidFormatter) ClassAssign(id, className string) string {
	return fmt.Sprintf("class %s %s", id, className)
}

// --- PlantUML formatter ---

type plantumlFormatter struct{}

func (plantumlFormatter) Header() string {
	return `@startuml
skinparam component {
  BackgroundColor #e8a838
  FontColor #14110d
  BorderColor #4a4030
}
skinparam arrow {
  Color #9a8d78
}
skinparam defaultTextAlignment left`
}

func (plantumlFormatter) Footer() string { return "@enduml" }

func (plantumlFormatter) NodeID(name string) string {
	return sanitizeDiagramID(name)
}

func (plantumlFormatter) NodeDecl(id, label string) string {
	return fmt.Sprintf(`component "%s" as %s`, plantumlLabel(label), id)
}

func (plantumlFormatter) EdgeDecl(fromID, toID string) string {
	return fmt.Sprintf("%s --> %s", fromID, toID)
}

func (plantumlFormatter) ClassAssign(_, _ string) string {
	// PlantUML doesn't support Mermaid-style class assignment.
	return ""
}
