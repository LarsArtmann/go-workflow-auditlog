package viz

import (
	"fmt"
	"io"
)

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
