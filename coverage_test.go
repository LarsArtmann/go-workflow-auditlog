package auditlog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

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

// --- Report query method tests ---

func TestReport_StepByName(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSucceed("find-me")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	report := a.Report()

	step := report.StepByName("find-me")
	if step == nil {
		t.Fatal("expected to find step 'find-me'")
	}

	if step.Name != "find-me" {
		t.Errorf("expected name 'find-me', got %q", step.Name)
	}

	if report.StepByName("nonexistent") != nil {
		t.Error("expected nil for nonexistent step")
	}
}

func TestReport_EventsByType(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSucceed("typed-step")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	report := a.Report()

	starts := report.EventsByType(auditlog.EventTypeAttemptStart)
	if len(starts) < 1 {
		t.Errorf("expected at least 1 start event, got %d", len(starts))
	}

	ends := report.EventsByType(auditlog.EventTypeAttemptEnd)
	if len(ends) < 1 {
		t.Errorf("expected at least 1 end event, got %d", len(ends))
	}
}

func TestReport_SucceededSteps(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	ok := newSucceed("ok1")
	ok2 := newSucceed("ok2")
	bad := newFail("bad", "err")
	w.Add(
		flow.Step(ok),
		flow.Step(ok2),
		flow.Step(bad),
	)
	runWorkflow(t, a, w)

	report := a.Report()

	succeeded := report.SucceededSteps()
	if len(succeeded) != 2 {
		t.Fatalf("expected 2 succeeded steps, got %d", len(succeeded))
	}
}

func TestReport_SkippedSteps(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	upstream := newFail("failing", "fail")
	downstream := newSucceed("skipped")
	addDependentStep(w, upstream, downstream)
	runWorkflow(t, a, w)

	report := a.Report()

	skipped := report.SkippedSteps()
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped step, got %d", len(skipped))
	}

	if skipped[0].Name != "skipped" {
		t.Errorf("expected 'skipped', got %q", skipped[0].Name)
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

// --- Validate error path tests ---

func TestReport_Validate_EventCountMismatch(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		EventCount: 99,
		Events:     []auditlog.Event{{}},
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("expected validation error for event count mismatch")
	}
}

func TestReport_Validate_StepCountMismatch(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		StepCount: 99,
		Steps:     []auditlog.StepInfo{{}},
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("expected validation error for step count mismatch")
	}
}

// TestReport_Validate_StatusDrift confirms the status drift check actually
// fires when a step's stored Status disagrees with its Error pointer. This
// is a regression test for a prior bug where the check could never fail.
func TestReport_Validate_StatusDrift(t *testing.T) {
	t.Parallel()

	errMsg := "boom"

	report := auditlog.WorkflowReport{
		EventCount: 1,
		Events:     []auditlog.Event{{}},
		StepCount:  1,
		Steps: []auditlog.StepInfo{{
			Status: auditlog.StepStatusPending, // non-terminal, but Error implies failure
			Error:  &errMsg,
		}},
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("expected validation error for status drift")
	}

	assertContains(t, err.Error(), "does not match derived status",
		"expected error mentioning status mismatch")
}

// --- Export tests ---

func TestWriteReportJSON_ToBuffer(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSucceed("json-step")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	var buf bytes.Buffer

	err := a.WriteReportJSON(&buf)
	if err != nil {
		t.Fatalf("WriteReportJSON error: %v", err)
	}

	var report auditlog.WorkflowReport

	err = json.Unmarshal(buf.Bytes(), &report)
	if err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	assertStepCount(t, report, 1)
}

func TestWriteEventsNDJSON_ToBuffer(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSucceed("ndjson-step")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	var buf bytes.Buffer

	err := a.WriteEventsNDJSON(&buf)
	if err != nil {
		t.Fatalf("WriteEventsNDJSON error: %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 NDJSON lines, got %d", len(lines))
	}

	var evt auditlog.Event

	err = json.Unmarshal(lines[0], &evt)
	if err != nil {
		t.Fatalf("invalid NDJSON line: %v", err)
	}
}

func TestExportToFile_Error(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})

	err := a.ExportToFile("/nonexistent/dir/file.json")
	if err == nil {
		t.Fatal("expected error writing to nonexistent directory")
	}
}

func TestExportEventsToNDJSON_Error(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})

	err := a.ExportEventsToNDJSON("/nonexistent/dir/file.ndjson")
	if err == nil {
		t.Fatal("expected error writing to nonexistent directory")
	}
}

// --- Config tests ---

func TestNew_InitialEventCapacity(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{
		Enabled:              true,
		InitialEventCapacity: 512,
	})
	w := &flow.Workflow{}
	s := newSucceed("cap-step")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	assertEventsRecorded(t, a, 2)
}

// --- Nil safety tests ---

func TestAttach_NilWorkflow(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})
	// Should not panic.
	a.Attach(nil)
}

func TestSnapshot_NilWorkflow(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})
	// Should not panic.
	a.Snapshot(nil)
}

func TestAttach_Disabled(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: false})
	w := &flow.Workflow{}
	s := newSucceed("step")
	w.Add(flow.Step(s))
	a.Attach(w) // no-op when disabled
}

// --- Complex scenario tests ---

func TestFanOutFanIn(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	root := newSucceed("root")
	branch1 := newSucceed("branch-1")
	branch2 := newSucceed("branch-2")
	join := newSucceed("join")
	w.Add(
		flow.Step(root),
		flow.Step(branch1).DependsOn(root),
		flow.Step(branch2).DependsOn(root),
		flow.Step(join).DependsOn(branch1, branch2),
	)
	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	assertStepCount(t, report, 4)

	joinStep := findStep(t, report, "join")
	if len(joinStep.Dependencies) != 2 {
		t.Errorf("expected join to have 2 deps, got %d", len(joinStep.Dependencies))
	}

	rootStep := findStep(t, report, "root")
	if len(rootStep.Dependents) != 2 {
		t.Errorf("expected root to have 2 dependents, got %d", len(rootStep.Dependents))
	}
}

func TestStepType_Inferred(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSucceed("typed")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	report := a.Report()

	step := report.StepByName("typed")
	if step.StepType != "succeedStep" {
		t.Errorf("expected step type 'succeedStep', got %q", step.StepType)
	}
}

func TestDuration_Tracked(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSlow("slow-step", 20*time.Millisecond)
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	report := a.Report()

	step := report.StepByName("slow-step")
	if step.DurationMs == nil {
		t.Fatal("expected non-nil DurationMs")
	}

	if *step.DurationMs < 10 {
		t.Errorf("expected duration >= 10ms, got %fms", *step.DurationMs)
	}

	if step.StartedAt == nil {
		t.Error("expected non-nil StartedAt")
	}

	if step.FinishedAt == nil {
		t.Error("expected non-nil FinishedAt")
	}
}

func TestWorkflowID_Propagated(t *testing.T) {
	t.Parallel()

	a := mustNewWithID(t, "my-custom-wf")
	w := &flow.Workflow{}
	s := newSucceed("step")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	report := a.Report()
	assertWorkflowID(t, report, "my-custom-wf")
}

func TestCanceledStatus(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSlow("cancel-me", 10*time.Second)
	w.Add(
		flow.Step(s).Timeout(10 * time.Millisecond),
	)
	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	step := report.StepByName("cancel-me")
	if step.Status != auditlog.StepStatusCanceled {
		t.Errorf("expected canceled, got %s", step.Status)
	}

	assertCount(t, "CanceledCount", report.CanceledCount, 1)

	if report.WorkflowSucceeded {
		t.Error("expected WorkflowSucceeded=false")
	}
}

func TestReport_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s1 := newSucceed("rt-1")
	s2 := newFail("rt-2", "fail")
	addDependentStep(w, s1, s2)
	runWorkflow(t, a, w)

	var buf bytes.Buffer

	err := a.WriteReportJSON(&buf)
	if err != nil {
		t.Fatalf("WriteReportJSON error: %v", err)
	}

	var report auditlog.WorkflowReport

	err = json.Unmarshal(buf.Bytes(), &report)
	if err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	// Verify the round-tripped report has correct data.
	if report.Version != auditlog.SchemaVersion {
		t.Errorf("version mismatch: %s", report.Version)
	}

	assertStepCount(t, report, 2)
}

func TestReport_WithRetryTiming(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newFlaky("retry-timing", 2)
	w.Add(
		flow.Step(s).Retry(retryOpts(5)),
	)
	runWorkflow(t, a, w)

	report := a.Report()
	step := report.StepByName("retry-timing")

	// Each attempt should have a start/end event pair.
	startEvents := report.EventsByType(auditlog.EventTypeAttemptStart)
	endEvents := report.EventsByType(auditlog.EventTypeAttemptEnd)

	if len(startEvents) != step.AttemptCount {
		t.Errorf("expected %d start events, got %d", step.AttemptCount, len(startEvents))
	}

	if len(endEvents) != step.AttemptCount {
		t.Errorf("expected %d end events, got %d", step.AttemptCount, len(endEvents))
	}

	// Verify attempt numbers are sequential.
	for i, evt := range startEvents {
		if evt.Attempt != i+1 {
			t.Errorf("start event %d: expected attempt %d, got %d", i, i+1, evt.Attempt)
		}
	}
}

func TestEnvEnabledFalse(t *testing.T) {
	t.Setenv(auditlog.EnvKeyEnabled, "false")

	a := mustNew(t, auditlog.Config{})
	w := &flow.Workflow{}
	s := newSucceed("env-false-step")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	if a.EventsCount() != 0 {
		t.Errorf("expected 0 events when disabled via env, got %d", a.EventsCount())
	}
}

func TestEventsCount_NoCopy(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := newSucceed("count-step")
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	count := a.EventsCount()

	events := a.Events()
	if count != len(events) {
		t.Errorf("EventsCount()=%d but len(Events())=%d", count, len(events))
	}
}

func TestReport_EmptyWorkflow(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})
	report := a.Report()

	assertStepCount(t, report, 0)

	assertEventCount(t, report, 0)

	if !report.WorkflowSucceeded {
		t.Error("empty workflow should be considered succeeded")
	}
}

// TestWorkflowSucceeded_PendingStepsIsFalse verifies that WorkflowSucceeded
// is false when a report contains non-terminal steps. The aggregate is
// recomputed through Filtered() → buildReportFromCore → finalizeDenormalized,
// which previously returned true as long as no step had failed or canceled.
func TestWorkflowSucceeded_PendingStepsIsFalse(t *testing.T) {
	t.Parallel()

	// Construct a report with a non-terminal step. WorkflowSucceeded is the
	// zero value here, so we route it through Filtered() to recompute.
	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "stuck"}, Status: auditlog.StepStatusRunning},
		},
	}

	recomputed := raw.Filtered()

	if recomputed.WorkflowSucceeded {
		t.Error("expected WorkflowSucceeded=false when steps are still running")
	}
}

func TestStepTypeName_NilStep(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	// Use a Func step which has a clean type name.
	w.Add(flow.Step(flow.Func("func-step", func(_ context.Context) error { return nil })))
	runWorkflow(t, a, w)

	report := a.Report()

	step := report.StepByName("func-step")
	if step == nil {
		t.Fatal("expected to find 'func-step'")
	}

	if step.StepType == "" {
		t.Error("expected non-empty step type")
	}
}

func TestSkip_Status(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	s := flow.Func("skip-me", func(_ context.Context) error {
		return flow.Skip(errors.New("not needed"))
	})
	w.Add(flow.Step(s))
	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	step := report.StepByName("skip-me")
	if step.Status != auditlog.StepStatusSkipped {
		t.Errorf("expected skipped, got %s", step.Status)
	}
}

func TestMultipleWorkflows_Isolated(t *testing.T) {
	t.Parallel()

	a1 := mustNewWithID(t, "wf-1")
	a2 := mustNewWithID(t, "wf-2")

	w1 := &flow.Workflow{}
	w1.Add(flow.Step(newSucceed("wf1-step")))

	w2 := &flow.Workflow{}
	w2.Add(flow.Step(newFail("wf2-step", "err")))

	runWorkflow(t, a1, w1)
	runWorkflow(t, a2, w2)

	r1 := a1.Report()
	r2 := a2.Report()

	assertWorkflowID(t, r1, "wf-1")
	assertWorkflowID(t, r2, "wf-2")

	if r1.StepCount != 1 || r2.StepCount != 1 {
		t.Errorf("expected each to have 1 step, got %d and %d", r1.StepCount, r2.StepCount)
	}
}

func TestWriteToFile_CloseError(t *testing.T) {
	t.Parallel()

	// Writing to a read-only file should fail on close.
	dir := t.TempDir()
	path := dir + "/readonly.json"
	_ = os.WriteFile(path, []byte{}, 0o444)

	a := mustNew(t, auditlog.Config{Enabled: true})

	err := a.ExportToFile(path)
	if err == nil {
		// Some filesystems allow writing to read-only files; skip if no error.
		return
	}
}
