package auditlog

import (
	"fmt"
	"io"

	"github.com/larsartmann/go-output"
)

// writeRendered invokes render, then writes the resulting string to writer with
// a trailing newline. Both the render error and the write error are wrapped
// with the auditlog sentinels (ErrRenderFailed and ErrExportWriteFailed
// respectively). The format label is interpolated into the error message so
// callers can produce format-specific error text without leaking the
// underlying renderer name into user-facing messages.
func writeRendered(writer io.Writer, format string, render func() (string, error)) error {
	out, err := render()
	if err != nil {
		return fmt.Errorf("%w: render %s: %w", ErrRenderFailed, format, err)
	}

	_, err = fmt.Fprintln(writer, out)
	if err != nil {
		return fmt.Errorf("%w: write %s output: %w", ErrExportWriteFailed, format, err)
	}

	return nil
}

// writeRenderedTransformed is like writeRendered but applies an optional
// transform to the rendered string before writing. Used by diagram writers
// that need to post-process the output (e.g. Mermaid direction override).
func writeRenderedTransformed(
	writer io.Writer,
	format string,
	render func() (string, error),
	transform func(string) string,
) error {
	out, err := render()
	if err != nil {
		return fmt.Errorf("%w: render %s: %w", ErrRenderFailed, format, err)
	}

	if transform != nil {
		out = transform(out)
	}

	_, err = fmt.Fprintln(writer, out)
	if err != nil {
		return fmt.Errorf("%w: write %s output: %w", ErrExportWriteFailed, format, err)
	}

	return nil
}

// writeGraph writes the step DAG of r through the given pre-configured
// go-output GraphRenderer. It centralizes the SetNodes/SetEdges + writeRendered
// sequence shared by every graph-format Write method (Mermaid, PlantUML,
// Graphviz). Each caller is responsible for constructing the renderer and
// applying any format-specific configuration (e.g. SetCodeFence, SetGraphID)
// before invoking this helper — keeping those one-line differences visible at
// the call site rather than hiding them behind an interface-widening wrapper.
func writeGraph(writer io.Writer, r WorkflowReport, format string, renderer output.GraphRenderer) error {
	nodes, edges := buildGraph(r)

	renderer.SetNodes(nodes)
	renderer.SetEdges(edges)

	return writeRendered(writer, format, renderer.Render)
}
