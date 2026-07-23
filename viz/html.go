package viz

import (
	"fmt"
	"io"

	"github.com/larsartmann/go-output/daghtml"
)

// DashboardCSS returns the base dashboard CSS used by the static HTML export.
// Exported so the live server package can reuse the same theme.
func DashboardCSS() string {
	return dashboardCSS
}

// DashboardJS returns the dashboard JavaScript used by the static HTML export.
// Exported so downstream packages can inspect or reuse the rendering logic.
func DashboardJS() string {
	return dashboardJS
}

// BuildDAGHTML converts a WorkflowReport into a daghtml.DAG for interactive
// graph rendering. Exported so the live server package can build the same DAG
// model without duplicating the conversion logic.
func BuildDAGHTML(report WorkflowReport) daghtml.DAG {
	return buildDAGHTML(report)
}

// WriteHTML writes a self-contained interactive HTML dashboard to writer.
// The output is a single HTML file with embedded CSS and JavaScript — no
// external dependencies, no network requests. It can be opened directly in
// any modern browser or attached to an email/report.
func WriteHTML(r WorkflowReport, writer io.Writer) error {
	html, err := renderHTML(r)
	if err != nil {
		return err
	}

	_, err = writer.Write([]byte(html))
	if err != nil {
		return fmt.Errorf("%w: write HTML report: %w", ErrExportWriteFailed, err)
	}

	return nil
}

// WriteHTMLString returns the HTML dashboard as a string.
// Convenience wrapper around WriteHTML for in-memory use.
func WriteHTMLString(r WorkflowReport) (string, error) {
	return renderHTML(r)
}

// ExportHTML writes the HTML dashboard to path (atomic write via temp+rename).
func ExportHTML(r WorkflowReport, path string) error {
	return WriteToFile(path, func(w io.Writer) error {
		return WriteHTML(r, w)
	})
}
