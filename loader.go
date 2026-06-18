package auditlog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// LoadReport reads a JSON WorkflowReport from a file path.
// This is the inverse of ExportToFile.
func LoadReport(path string) (WorkflowReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return WorkflowReport{}, fmt.Errorf("open report %q: %w", path, err)
	}
	defer f.Close()

	return LoadReportFromReader(f)
}

// LoadReportFromReader reads a JSON WorkflowReport from any io.Reader.
func LoadReportFromReader(reader io.Reader) (WorkflowReport, error) {
	var report WorkflowReport

	decoder := json.NewDecoder(reader)

	err := decoder.Decode(&report)
	if err != nil {
		return WorkflowReport{}, fmt.Errorf("decode report: %w", err)
	}

	return report, nil
}

// LoadReportFromBytes parses a JSON WorkflowReport from a byte slice.
func LoadReportFromBytes(data []byte) (WorkflowReport, error) {
	var report WorkflowReport

	err := json.Unmarshal(data, &report)
	if err != nil {
		return WorkflowReport{}, fmt.Errorf("unmarshal report: %w", err)
	}

	return report, nil
}
