package auditlog

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// WriteCSV writes all steps as comma-separated values to the writer.
// The first row is a header. Pointer fields render as empty strings when nil.
// Dependencies and dependents are semicolon-separated step names.
func (r WorkflowReport) WriteCSV(writer io.Writer) error {
	return r.writeDelimited(writer, ',')
}

// WriteTSV writes all steps as tab-separated values to the writer.
// Identical to WriteCSV but with a tab delimiter for tools that prefer TSV.
func (r WorkflowReport) WriteTSV(writer io.Writer) error {
	return r.writeDelimited(writer, '\t')
}

func (r WorkflowReport) writeDelimited(writer io.Writer, comma rune) error {
	w := csv.NewWriter(writer)
	w.Comma = comma

	header := []string{
		"step_id", "step_name", "step_type",
		"status",
		"attempt_count", "max_attempts",
		"started_at", "finished_at", "duration_ms",
		"has_retry", "has_timeout",
		"error",
		"dependencies", "dependents",
	}

	err := w.Write(header)
	if err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	for _, step := range r.Steps {
		err := w.Write(stepToCSVRow(step))
		if err != nil {
			return fmt.Errorf("write step %q: %w", step.Name, err)
		}
	}

	w.Flush()

	err = w.Error()
	if err != nil {
		return fmt.Errorf("%w: flush delimited writer: %w", ErrExportWriteFailed, err)
	}

	return nil
}

// ExportCSV writes all steps as CSV to path.
func (r WorkflowReport) ExportCSV(path string) error {
	return WriteToFile(path, r.WriteCSV)
}

// ExportTSV writes all steps as TSV to path.
func (r WorkflowReport) ExportTSV(path string) error {
	return WriteToFile(path, r.WriteTSV)
}

func stepToCSVRow(s StepInfo) []string {
	return []string{
		strconv.Itoa(s.StepID),
		s.Name,
		s.StepType,
		string(s.Status),
		strconv.Itoa(s.AttemptCount),
		strconv.Itoa(s.MaxAttempts),
		formatTimePtr(s.StartedAt),
		formatTimePtr(s.FinishedAt),
		formatFloatPtr(s.DurationMs),
		strconv.FormatBool(s.HasRetry),
		strconv.FormatBool(s.HasTimeout),
		formatStrPtr(s.Error),
		formatStepRefs(s.Dependencies),
		formatStepRefs(s.Dependents),
	}
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.Format(time.RFC3339Nano)
}

func formatFloatPtr(f *float64) string {
	if f == nil {
		return ""
	}

	return strconv.FormatFloat(*f, 'f', -1, 64)
}

func formatStrPtr(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func formatStepRefs(refs []StepRef) string {
	if len(refs) == 0 {
		return ""
	}

	parts := make([]string, 0, len(refs))

	for _, ref := range refs {
		parts = append(parts, ref.Name)
	}

	return strings.Join(parts, ";")
}
