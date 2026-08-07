package auditlog

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"
)

var ErrMigrationEmptyInput = errors.New("migration input is empty")

var ErrMigrationMissingVersion = errors.New("migration input has no version field")

// RedriveReportStatuses calls DeriveStatus on every step in the report and
// updates the Status field to match. Used by MigrateReport (repair) and
// property-test fixture builders (sanitize).
func RedriveReportStatuses(report *WorkflowReport) {
	for idx := range report.Steps {
		report.Steps[idx].Status = report.Steps[idx].DeriveStatus()
	}
}

// MigrateReport takes a raw JSON byte slice representing a report exported
// by a previous schema version and returns a WorkflowReport compatible with
// the current SchemaVersion. Unknown fields are preserved through round-tripping.
//
// In addition to upgrading older schemas, MigrateReport always re-derives the
// denormalized count and aggregate fields (EventCount, StepCount, status
// counts, durations, WorkflowSucceeded) from the actual data. This means
// current-schema input is also repaired: stale or hand-edited reports that
// would fail Validate() are normalized so the returned WorkflowReport is
// always valid. The implied contract is "repair/normalize -> current", not
// just "upgrade old -> current".
func MigrateReport(data []byte) (WorkflowReport, error) {
	if len(data) == 0 {
		return WorkflowReport{}, ErrMigrationEmptyInput
	}

	var report WorkflowReport

	err := json.Unmarshal(data, &report)
	if err != nil {
		return WorkflowReport{}, fmt.Errorf("unmarshal report: %w", err)
	}

	if report.Version == "" {
		return WorkflowReport{}, ErrMigrationMissingVersion
	}

	exportedAt := report.ExportedAt
	if exportedAt.IsZero() {
		exportedAt = time.Now()
	}

	RedriveReportStatuses(&report)

	return buildReportFromCore(
		SchemaVersion,
		report.WorkflowID,
		report.RunID,
		exportedAt,
		report.DroppedEventCount,
		report.Events,
		report.Steps,
	), nil
}
