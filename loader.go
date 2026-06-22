package auditlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrReportLoadFailed wraps errors encountered while loading or decoding a
// report from a file, reader, or byte slice. Classified as Transient — the
// caller may retry (e.g. the file might be temporarily locked or mid-write).
// Consumers can match on it with [errors.Is].
var ErrReportLoadFailed = errors.New("report load failed")

// LoadReport reads a JSON WorkflowReport from a file path.
// This is the inverse of ExportJSON.
func LoadReport(path string) (WorkflowReport, error) {
	f, err := os.Open(path) //nolint:gosec // path is user-provided by design.
	if err != nil {
		return WorkflowReport{}, fmt.Errorf("%w: open %q: %w", ErrReportLoadFailed, path, err)
	}
	defer func() {
		_ = f.Close()
	}()

	return LoadReportFromReader(f)
}

// LoadReportFromReader reads a JSON WorkflowReport from any io.Reader.
func LoadReportFromReader(reader io.Reader) (WorkflowReport, error) {
	var report WorkflowReport

	decoder := json.NewDecoder(reader)

	err := decoder.Decode(&report)
	if err != nil {
		return WorkflowReport{}, fmt.Errorf("%w: decode: %w", ErrReportLoadFailed, err)
	}

	return report, nil
}

// LoadReportFromBytes parses a JSON WorkflowReport from a byte slice.
func LoadReportFromBytes(data []byte) (WorkflowReport, error) {
	var report WorkflowReport

	err := json.Unmarshal(data, &report)
	if err != nil {
		return WorkflowReport{}, fmt.Errorf("%w: unmarshal: %w", ErrReportLoadFailed, err)
	}

	return report, nil
}
