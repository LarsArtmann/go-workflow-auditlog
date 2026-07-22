package auditlog_test

import (
	"testing"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// --- Event method tests ---

func TestEvent_ConvenienceMethods(t *testing.T) {
	t.Parallel()

	dur := 5.5
	errMsg := "boom"

	startEvent := auditlog.Event{
		EventType:  auditlog.EventTypeAttemptStart,
		Phase:      auditlog.PhaseBefore,
		StepRef:    auditlog.StepRef{Name: "step-a"},
		DurationMs: &dur,
	}

	endEvent := auditlog.Event{
		EventType: auditlog.EventTypeAttemptEnd,
		Phase:     auditlog.PhaseAfter,
		Error:     &errMsg,
		Status:    auditlog.StepStatusFailed,
	}

	if !startEvent.IsAttemptStart() {
		t.Error("expected IsAttemptStart=true")
	}

	if startEvent.IsAttemptEnd() {
		t.Error("expected IsAttemptEnd=false")
	}

	if !endEvent.IsAttemptEnd() {
		t.Error("expected IsAttemptEnd=true")
	}

	if !startEvent.IsBefore() {
		t.Error("expected IsBefore=true")
	}

	if !endEvent.IsAfter() {
		t.Error("expected IsAfter=true")
	}

	if !endEvent.HasError() {
		t.Error("expected HasError=true")
	}

	if startEvent.HasError() {
		t.Error("expected HasError=false")
	}

	if startEvent.Duration() != 5.5 {
		t.Errorf("expected Duration=5.5, got %f", startEvent.Duration())
	}

	if endEvent.Duration() != 0 {
		t.Errorf("expected Duration=0, got %f", endEvent.Duration())
	}
}

func TestEvent_Label(t *testing.T) {
	t.Parallel()

	if auditlog.EventTypeAttemptStart.Label() != "Attempt Start" {
		t.Errorf("unexpected label: %s", auditlog.EventTypeAttemptStart.Label())
	}

	if auditlog.EventTypeAttemptEnd.Label() != "Attempt End" {
		t.Errorf("unexpected label: %s", auditlog.EventTypeAttemptEnd.Label())
	}

	unknown := auditlog.EventType("unknown")
	if unknown.Label() != "" {
		t.Errorf("expected empty label for unknown type")
	}
}

func TestEvent_Color(t *testing.T) {
	t.Parallel()

	if auditlog.EventTypeAttemptStart.Color() == "" {
		t.Error("expected non-empty color")
	}

	unknown := auditlog.EventType("unknown")
	if unknown.Color() != "" {
		t.Error("expected empty color for unknown type")
	}
}

// --- StepStatus method tests ---

func TestStepStatus_Methods(t *testing.T) {
	t.Parallel()

	if auditlog.StepStatusSucceeded.String() != "succeeded" {
		t.Errorf("expected 'succeeded', got %s", auditlog.StepStatusSucceeded.String())
	}

	if auditlog.StepStatusSucceeded.Label() != "Succeeded" {
		t.Errorf("expected 'Succeeded', got %s", auditlog.StepStatusSucceeded.Label())
	}

	if !auditlog.StepStatusFailed.IsTerminal() {
		t.Error("expected Failed to be terminal")
	}

	if !auditlog.StepStatusSkipped.IsTerminal() {
		t.Error("expected Skipped to be terminal")
	}

	if auditlog.StepStatusPending.IsTerminal() {
		t.Error("expected Pending to NOT be terminal")
	}

	if auditlog.StepStatusRunning.IsTerminal() {
		t.Error("expected Running to NOT be terminal")
	}

	if !auditlog.StepStatusFailed.IsError() {
		t.Error("expected Failed to be error")
	}

	if !auditlog.StepStatusCanceled.IsError() {
		t.Error("expected Canceled to be error")
	}

	if auditlog.StepStatusSucceeded.IsError() {
		t.Error("expected Succeeded to NOT be error")
	}

	if auditlog.StepStatusSucceeded.Icon() == "" {
		t.Error("expected non-empty icon")
	}
}

// --- StepInfo method tests ---

func TestStepInfo_HasError(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newFail("fail-step", "err")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	report := a.Report()

	step := report.StepByName("fail-step")
	if !step.HasError() {
		t.Error("expected HasError=true")
	}

	okStep := report.StepByName("fail-step")

	okStep.Error = nil
	if okStep.HasError() {
		t.Error("expected HasError=false after clearing error")
	}
}

func TestStepInfo_DeriveStatus_Terminal(t *testing.T) {
	t.Parallel()

	s := auditlog.StepInfo{Status: auditlog.StepStatusSucceeded}
	if s.DeriveStatus() != auditlog.StepStatusSucceeded {
		t.Error("expected terminal status preserved")
	}
}

func TestStepInfo_DeriveStatus_FromError(t *testing.T) {
	t.Parallel()

	errMsg := "fail"

	s := auditlog.StepInfo{
		Status: auditlog.StepStatusRunning,
		Error:  &errMsg,
	}
	if s.DeriveStatus() != auditlog.StepStatusFailed {
		t.Error("expected failed from error")
	}
}

// TestCoverage_StepInfo_Type covers StepInfo.Type() (was 0%).
func TestCoverage_StepInfo_Type(t *testing.T) {
	t.Parallel()

	step := auditlog.StepInfo{
		StepRef: auditlog.StepRef{Name: "x", StepType: "FetchStep"},
	}

	if step.Type() != "FetchStep" {
		t.Errorf("expected Type()='FetchStep', got %q", step.Type())
	}
}

// TestCoverage_StepStatus_IsKnown covers StepStatus.IsKnown() (was 0%).
func TestCoverage_StepStatus_IsKnown(t *testing.T) {
	t.Parallel()

	if !auditlog.StepStatusSucceeded.IsKnown() {
		t.Error("expected Succeeded to be known")
	}

	unknown := auditlog.StepStatus("bogus")
	if unknown.IsKnown() {
		t.Error("expected bogus status to be unknown")
	}
}

// TestCoverage_RunID_StringAndIsEmpty covers RunID.String() and
// RunID.IsEmpty() (both were 0%).
func TestCoverage_RunID_StringAndIsEmpty(t *testing.T) {
	t.Parallel()

	id := auditlog.RunID("abc123")
	if id.String() != "abc123" {
		t.Errorf("expected String()='abc123', got %q", id.String())
	}

	if id.IsEmpty() {
		t.Error("expected non-empty RunID to not be empty")
	}

	var empty auditlog.RunID
	if !empty.IsEmpty() {
		t.Error("expected zero-value RunID to be empty")
	}
}

// TestCoverage_StepStatus_Color_Unknown covers the unknown-status branch of
// StepStatus.Color() (was 66.7%).
func TestCoverage_StepStatus_Color_Unknown(t *testing.T) {
	t.Parallel()

	unknown := auditlog.StepStatus("bogus")
	fill, font := unknown.Color()

	if fill != "" || font != "" {
		t.Errorf("expected empty colors for unknown status, got fill=%q font=%q", fill, font)
	}
}
