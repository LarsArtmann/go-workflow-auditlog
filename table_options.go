package auditlog

import (
	"strconv"
	"strings"
)

// TableColumn identifies a column available in the step summary table.
type TableColumn int

const (
	// ColumnStep is the step display name.
	ColumnStep TableColumn = iota
	// ColumnStatus is the step execution status (succeeded, failed, etc.).
	ColumnStatus
	// ColumnDuration is the step execution duration in milliseconds.
	ColumnDuration
	// ColumnAttempts is the number of execution attempts made.
	ColumnAttempts
	// ColumnMaxAttempts is the configured maximum number of attempts.
	ColumnMaxAttempts
	// ColumnRetry indicates whether the step has retry configuration.
	ColumnRetry
	// ColumnTimeout indicates whether the step has a timeout configured.
	ColumnTimeout
	// ColumnError is the error message from the step (empty if succeeded).
	ColumnError
	// ColumnType is the step's Go type name.
	ColumnType
	// ColumnDependencies is a comma-separated list of dependency step names.
	ColumnDependencies
)

// DefaultTableColumns is the column set used when WithColumns is not called.
// It matches the original hardcoded layout for backward compatibility.
//
// Treat as read-only: mutating this slice will corrupt the default for all
// subsequent calls. Use WithColumns to customize column selection.
//
//nolint:gochecknoglobals // Read-only default; internal callers always copy before use.
var DefaultTableColumns = []TableColumn{
	ColumnStep,
	ColumnStatus,
	ColumnDuration,
	ColumnAttempts,
	ColumnRetry,
	ColumnTimeout,
	ColumnError,
}

// String returns the column name for debug/logging (e.g. "Step", "Status").
func (c TableColumn) String() string {
	if def, ok := columnDefs[c]; ok {
		return def.header
	}

	return "Unknown"
}

// defaultColumnsCopy returns a fresh copy of DefaultTableColumns so callers
// can freely mutate the returned slice without corrupting the package default.
func defaultColumnsCopy() []TableColumn {
	return append([]TableColumn(nil), DefaultTableColumns...)
}

// AllTableColumns returns every available table column in canonical order.
func AllTableColumns() []TableColumn {
	return []TableColumn{
		ColumnStep,
		ColumnStatus,
		ColumnDuration,
		ColumnAttempts,
		ColumnMaxAttempts,
		ColumnRetry,
		ColumnTimeout,
		ColumnError,
		ColumnType,
		ColumnDependencies,
	}
}

// columnDefinition pairs a header label with a cell extractor function.
type columnDefinition struct {
	header  string
	extract func(StepInfo) string
}

// columnDefs is the single source of truth mapping TableColumn values to
// their header text and data extraction logic. Adding a new column is a
// two-line change: a const above and an entry here.
//
//nolint:gochecknoglobals // Lookup table, treated as immutable after init.
var columnDefs = map[TableColumn]columnDefinition{
	ColumnStep: {
		header:  "Step",
		extract: func(s StepInfo) string { return s.Name },
	},
	ColumnStatus: {
		header:  "Status",
		extract: func(s StepInfo) string { return string(s.Status) },
	},
	ColumnDuration: {
		header:  "Duration",
		extract: extractDurationCell,
	},
	ColumnAttempts: {
		header:  "Attempts",
		extract: func(s StepInfo) string { return strconv.Itoa(s.AttemptCount) },
	},
	ColumnMaxAttempts: {
		header:  "Max Attempts",
		extract: func(s StepInfo) string { return strconv.Itoa(s.MaxAttempts) },
	},
	ColumnRetry: {
		header:  "Retry",
		extract: func(s StepInfo) string { return strconv.FormatBool(s.HasRetry) },
	},
	ColumnTimeout: {
		header:  "Timeout",
		extract: func(s StepInfo) string { return strconv.FormatBool(s.HasTimeout) },
	},
	ColumnError: {
		header:  "Error",
		extract: extractErrorCell,
	},
	ColumnType: {
		header:  "Type",
		extract: func(s StepInfo) string { return s.StepType },
	},
	ColumnDependencies: {
		header:  "Dependencies",
		extract: extractDependenciesCell,
	},
}

func extractDurationCell(s StepInfo) string {
	if s.DurationMs != nil && *s.DurationMs > 0 {
		return strconv.FormatFloat(*s.DurationMs, 'f', 2, 64) + "ms"
	}

	return ""
}

func extractErrorCell(s StepInfo) string {
	if s.Error != nil {
		return *s.Error
	}

	return ""
}

func extractDependenciesCell(s StepInfo) string {
	if len(s.Dependencies) == 0 {
		return ""
	}

	names := make([]string, 0, len(s.Dependencies))
	for _, dep := range s.Dependencies {
		names = append(names, dep.Name)
	}

	return strings.Join(names, ", ")
}

// TableOption configures step summary table output.
type TableOption func(*tableConfig)

type tableConfig struct {
	columns []TableColumn
}

// WithColumns selects which columns appear in the table output.
// Columns appear in the order specified. If not called, DefaultTableColumns
// is used (Step, Status, Duration, Attempts, Retry, Timeout, Error).
//
// Example: show only step name and status:
//
//	report.WriteTable(w, output.FormatMarkdown, output.RenderOptions{},
//	    auditlog.WithColumns(auditlog.ColumnStep, auditlog.ColumnStatus))
func WithColumns(cols ...TableColumn) TableOption {
	return func(c *tableConfig) { c.columns = cols }
}

func applyTableOpts(opts []TableOption) tableConfig {
	var cfg tableConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	if len(cfg.columns) == 0 {
		cfg.columns = defaultColumnsCopy()
	}

	return cfg
}
