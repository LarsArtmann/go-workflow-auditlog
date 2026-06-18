package auditlog_test

import (
	"testing"

	"github.com/larsartmann/go-workflow-auditlog"
)

// Test all StepStatus display methods for completeness.
func TestStepStatus_AllLabels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status auditlog.StepStatus
		label  string
	}{
		{auditlog.StepStatusPending, "Pending"},
		{auditlog.StepStatusRunning, "Running"},
		{auditlog.StepStatusSucceeded, "Succeeded"},
		{auditlog.StepStatusFailed, "Failed"},
		{auditlog.StepStatusCanceled, "Canceled"},
		{auditlog.StepStatusSkipped, "Skipped"},
	}

	for _, tc := range cases {
		if tc.status.Label() != tc.label {
			t.Errorf("%s: expected label %q, got %q", tc.status, tc.label, tc.status.Label())
		}

		if tc.status.Icon() == "" {
			t.Errorf("%s: expected non-empty icon", tc.status)
		}
	}

	// Unknown status.
	if auditlog.StepStatus("unknown").Label() != "" {
		t.Error("expected empty label for unknown status")
	}

	if auditlog.StepStatus("unknown").Icon() != "" {
		t.Error("expected empty icon for unknown status")
	}
}

func TestEventType_AllColors(t *testing.T) {
	t.Parallel()

	if auditlog.EventTypeAttemptEnd.Color() == "" {
		t.Error("expected non-empty color for AttemptEnd")
	}
}

func TestStepStatus_String(t *testing.T) {
	t.Parallel()

	for _, s := range []auditlog.StepStatus{
		auditlog.StepStatusPending,
		auditlog.StepStatusRunning,
		auditlog.StepStatusSucceeded,
		auditlog.StepStatusFailed,
		auditlog.StepStatusCanceled,
		auditlog.StepStatusSkipped,
	} {
		if s.String() == "" {
			t.Errorf("expected non-empty string for %s", s.Label())
		}
	}
}

func TestDeriveStatus_PendingNoError(t *testing.T) {
	t.Parallel()

	s := auditlog.StepInfo{Status: auditlog.StepStatusPending}
	if s.DeriveStatus() != auditlog.StepStatusPending {
		t.Errorf("expected pending, got %s", s.DeriveStatus())
	}
}
