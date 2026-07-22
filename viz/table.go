package viz

import (
	"fmt"
	"io"
	"strings"

	"github.com/larsartmann/go-output"
	// Blank imports register table data renderers for RenderTable dispatch.
	_ "github.com/larsartmann/go-output/delimited"
	_ "github.com/larsartmann/go-output/markdown"
	_ "github.com/larsartmann/go-output/serialization"
	_ "github.com/larsartmann/go-output/table"
)

// buildTableData converts a WorkflowReport into go-output Table.
// When columns is empty, DefaultTableColumns is used.
func buildTableData(r WorkflowReport, columns []TableColumn) *output.Table {
	if len(columns) == 0 {
		columns = defaultColumnsCopy()
	}

	headers := make([]string, 0, len(columns))
	for _, col := range columns {
		def, ok := columnDefs[col]
		if !ok {
			continue
		}

		headers = append(headers, def.header)
	}

	data := output.NewTable(headers)

	for _, step := range r.Steps {
		row := make([]string, 0, len(columns))
		for _, col := range columns {
			def, ok := columnDefs[col]
			if !ok {
				continue
			}

			row = append(row, def.extract(step))
		}

		data.AddRow(row)
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
//
// The tableOpts parameter controls which columns appear. By default all 7
// standard columns are shown; use WithColumns to customize:
//
//	viz.WriteTable(report, w, output.FormatCSV, output.RenderOptions{},
//	    viz.WithColumns(viz.ColumnStep, viz.ColumnStatus))
func WriteTable(
	r WorkflowReport,
	writer io.Writer,
	format output.Format,
	opts output.RenderOptions,
	tableOpts ...TableOption,
) error {
	cfg := applyTableOpts(tableOpts)

	data := buildTableData(r, cfg.columns)

	opts.Writer = writer

	err := output.RenderTable(data, format, opts)
	if err != nil {
		return fmt.Errorf("%w: render table: %w", ErrRenderFailed, err)
	}

	return nil
}

// WriteTableString returns the step summary table as a string in the
// specified format. See WriteTable for supported formats and options.
func WriteTableString(
	r WorkflowReport,
	format output.Format,
	opts output.RenderOptions,
	tableOpts ...TableOption,
) (string, error) {
	var buf strings.Builder

	err := WriteTable(r, &buf, format, opts, tableOpts...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ExportTable writes the step summary table to path.
func ExportTable(
	r WorkflowReport,
	path string,
	format output.Format,
	opts output.RenderOptions,
	tableOpts ...TableOption,
) error {
	return WriteToFile(path, func(w io.Writer) error {
		return WriteTable(r, w, format, opts, tableOpts...)
	})
}
