package auditlog_test

import (
	"encoding/json/v2"
	"errors"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestMigrateReport_EmptyInput(t *testing.T) {
	t.Parallel()

	_, err := auditlog.MigrateReport(nil)
	if !errors.Is(err, auditlog.ErrMigrationEmptyInput) {
		t.Errorf("expected ErrMigrationEmptyInput, got %v", err)
	}
}

func TestMigrateReport_MissingVersion(t *testing.T) {
	t.Parallel()

	input := `{"workflow_id":"test","steps":[]}`

	_, err := auditlog.MigrateReport([]byte(input))
	if !errors.Is(err, auditlog.ErrMigrationMissingVersion) {
		t.Errorf("expected ErrMigrationMissingVersion, got %v", err)
	}
}

func TestMigrateReport_NormalizesCurrentSchema(t *testing.T) {
	t.Parallel()

	input := `{
		"version": "0.1.0",
		"workflow_id": "test-wf",
		"exported_at": "2026-01-01T00:00:00Z",
		"event_count": 999,
		"step_count": 999,
		"events": [
			{"step_name":"fetch","sequence":1,"timestamp":"2026-01-01T00:00:00Z","event_type":"attempt_start","phase":"before","attempt":1},
			{"step_name":"fetch","sequence":2,"timestamp":"2026-01-01T00:00:00.01Z","event_type":"attempt_end","phase":"after","attempt":1,"duration_ms":10,"status":"succeeded"}
		],
		"steps": [
			{"step_name":"fetch","status":"succeeded","attempt_count":1,"duration_ms":10,"has_retry":false,"has_timeout":false}
		]
	}`

	report, err := auditlog.MigrateReport([]byte(input))
	if err != nil {
		t.Fatalf("MigrateReport: %v", err)
	}

	if report.Version != auditlog.SchemaVersion {
		t.Errorf("version: want %s, got %s", auditlog.SchemaVersion, report.Version)
	}

	if report.EventCount != 2 {
		t.Errorf("EventCount: want 2 (re-derived), got %d", report.EventCount)
	}

	if report.StepCount != 1 {
		t.Errorf("StepCount: want 1 (re-derived), got %d", report.StepCount)
	}

	if report.SucceededCount != 1 {
		t.Errorf("SucceededCount: want 1, got %d", report.SucceededCount)
	}

	if !report.WorkflowSucceeded {
		t.Error("expected WorkflowSucceeded=true for all-succeeded steps")
	}
}

func TestMigrateReport_Idempotent(t *testing.T) {
	t.Parallel()

	input := `{
		"version": "0.1.0",
		"workflow_id": "test-wf",
		"exported_at": "2026-01-01T00:00:00Z",
		"events": [],
		"steps": []
	}`

	report1, err := auditlog.MigrateReport([]byte(input))
	if err != nil {
		t.Fatalf("first MigrateReport: %v", err)
	}

	data, err := json.Marshal(report1)
	if err != nil {
		t.Fatalf("marshal report1: %v", err)
	}

	report2, err := auditlog.MigrateReport(data)
	if err != nil {
		t.Fatalf("second MigrateReport: %v", err)
	}

	if report1.EventCount != report2.EventCount {
		t.Errorf("EventCount changed: %d -> %d", report1.EventCount, report2.EventCount)
	}

	if report1.StepCount != report2.StepCount {
		t.Errorf("StepCount changed: %d -> %d", report1.StepCount, report2.StepCount)
	}

	if report1.WorkflowSucceeded != report2.WorkflowSucceeded {
		t.Errorf("WorkflowSucceeded changed: %v -> %v", report1.WorkflowSucceeded, report2.WorkflowSucceeded)
	}
}
