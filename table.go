package auditlog

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/larsartmann/go-output"
	// Blank imports register table data renderers for RenderTableData dispatch.
	_ "github.com/larsartmann/go-output/delimited"
	_ "github.com/larsartmann/go-output/markdown"
	_ "github.com/larsartmann/go-output/serialization"
	_ "github.com/larsartmann/go-output/table"
)

// buildTableData converts a WorkflowReport into go-output TableData.
// Columns: Step, Status, Duration, Attempts, Retry, Timeout, Error.
func (r WorkflowReport) buildTableData() *output.TableData {
	data := output.NewTableData([]string{"Step", "Status", "Duration", "Attempts", "Retry", "Timeout", "Error"})

	for _, step := range r.Steps {
		errStr := ""
		if step.Error != nil {
			errStr = *step.Error
		}

		durStr := ""
		if step.DurationMs != nil && *step.DurationMs > 0 {
			durStr = fmt.Sprintf("%.2fms", *step.DurationMs)
		}

		retryStr := strconv.FormatBool(step.HasRetry)
		timeoutStr := strconv.FormatBool(step.HasTimeout)

		data.AddRow([]string{
			step.Name,
			string(step.Status),
			durStr,
			strconv.Itoa(step.AttemptCount),
			retryStr,
			timeoutStr,
			errStr,
		})
	}

	return data
}

// WriteTable writes the step summary as a table in the specified format.
// Supported formats (when respective sub-modules are imported): table,
// json, csv, tsv, markdown, xml, d2, yaml, html, tree, mermaid, dot,
// jsonl, asciidoc, toml, plantuml.
//
// The opts parameter controls color mode, title, and output destination.
// Pass output.RenderOptions{} for defaults.
func (r WorkflowReport) WriteTable(writer io.Writer, format output.Format, opts output.RenderOptions) error {
	data := r.buildTableData()

	opts.Writer = writer

	err := output.RenderTableData(data, format, opts)
	if err != nil {
		return fmt.Errorf("%w: render table: %w", ErrRenderFailed, err)
	}

	return nil
}

// WriteTableString returns the step summary table as a string in the
// specified format. See WriteTable for supported formats.
func (r WorkflowReport) WriteTableString(format output.Format, opts output.RenderOptions) (string, error) {
	var buf strings.Builder

	err := r.WriteTable(&buf, format, opts)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
