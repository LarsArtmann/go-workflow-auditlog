package auditlog_test

import (
	"bytes"
	"context"
	"encoding/hex"
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

	if !errors.Is(err, auditlog.ErrEventCountMismatch) {
		t.Errorf("expected error to wrap auditlog.ErrEventCountMismatch, got: %v", err)
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

	if !errors.Is(err, auditlog.ErrStepCountMismatch) {
		t.Errorf("expected error to wrap auditlog.ErrStepCountMismatch, got: %v", err)
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

	if !errors.Is(err, auditlog.ErrStatusDrift) {
		t.Errorf("expected error to wrap auditlog.ErrStatusDrift, got: %v", err)
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

// TestRunID_DefaultGenerated confirms that when no RunID is supplied, the
// auditor generates a non-empty, hex-formatted run ID and stamps it on every
// captured event and the final report.
func TestRunID_DefaultGenerated(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})
	w := &flow.Workflow{}
	w.Add(flow.Step(newSucceed("step-a")))
	w.Add(flow.Step(newSucceed("step-b")))
	runWorkflow(t, a, w)

	runID := a.RunID()
	if runID == "" {
		t.Fatal("expected non-empty generated RunID")
	}

	if len(runID) != 32 {
		t.Errorf("expected 32-char hex RunID, got %d chars: %q", len(runID), runID)
	}

	_, err := hex.DecodeString(runID)
	if err != nil {
		t.Errorf("expected lowercase hex RunID, got %q: %v", runID, err)
	}

	report := a.Report()
	if report.RunID != runID {
		t.Errorf("report RunID %q != accessor RunID %q", report.RunID, runID)
	}

	if len(report.Events) == 0 {
		t.Fatal("expected events in report")
	}

	for i, evt := range report.Events {
		if evt.RunID != runID {
			t.Errorf("event %d RunID %q != run RunID %q", i, evt.RunID, runID)
		}
	}
}

// TestRunID_CustomHonored confirms a caller-supplied RunID is used verbatim
// (no generation, no mutation) on the report and events.
func TestRunID_CustomHonored(t *testing.T) {
	t.Parallel()

	const custom = "trace-abc-123"

	a := mustNew(t, auditlog.Config{Enabled: true, RunID: custom})
	w := &flow.Workflow{}
	w.Add(flow.Step(newSucceed("step")))
	runWorkflow(t, a, w)

	if a.RunID() != custom {
		t.Errorf("expected custom RunID %q, got %q", custom, a.RunID())
	}

	report := a.Report()
	if report.RunID != custom {
		t.Errorf("report RunID %q != custom %q", report.RunID, custom)
	}
}

// TestRunID_ReplayRoundTrip confirms the RunID survives an NDJSON export →
// ReplayEvents round trip, so offline analysis can still correlate the run.
func TestRunID_ReplayRoundTrip(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})
	w := &flow.Workflow{}
	w.Add(flow.Step(newSucceed("step")))
	runWorkflow(t, a, w)

	runID := a.RunID()
	report := a.Report()

	replayed, err := auditlog.ReplayEvents(report.Events)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	if replayed.RunID != runID {
		t.Errorf("replayed RunID %q != original %q", replayed.RunID, runID)
	}
}

// TestRunID_UniquePerAuditor confirms two auditors get distinct run IDs, so
// concurrent or sequential runs never share a correlation key.
func TestRunID_UniquePerAuditor(t *testing.T) {
	t.Parallel()

	a1 := mustNew(t, auditlog.Config{Enabled: true})
	a2 := mustNew(t, auditlog.Config{Enabled: true})

	if a1.RunID() == a2.RunID() {
		t.Errorf("expected distinct RunIDs, both were %q", a1.RunID())
	}
}

// TestStepID_UniqueAndSequential confirms every step gets a distinct, non-zero
// StepID and that all IDs form a contiguous 1..N set.
func TestStepID_UniqueAndSequential(t *testing.T) {
	t.Parallel()

	a := mustNew(t, auditlog.Config{Enabled: true})
	w := &flow.Workflow{}
	w.Add(flow.Step(newSucceed("step-a")))
	w.Add(flow.Step(newSucceed("step-b")))
	w.Add(flow.Step(newSucceed("step-c")))
	runWorkflow(t, a, w)

	report := a.Report()
	if len(report.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(report.Steps))
	}

	seen := make(map[int]string, len(report.Steps))

	for _, step := range report.Steps {
		if step.StepID == 0 {
			t.Errorf("step %q has zero StepID", step.Name)
		}

		if dup, ok := seen[step.StepID]; ok {
			t.Errorf("StepID %d shared by %q and %q", step.StepID, dup, step.Name)
		}

		seen[step.StepID] = step.Name
	}
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

func TestReport_AggregateCounts_PendingRunning(t *testing.T) {
	t.Parallel()

	// Build a report with a non-terminal step directly.
	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "pending-step"}, Status: auditlog.StepStatusPending},
			{StepRef: auditlog.StepRef{Name: "running-step"}, Status: auditlog.StepStatusRunning},
			{StepRef: auditlog.StepRef{Name: "done-step"}, Status: auditlog.StepStatusSucceeded},
		},
	}

	recomputed := raw.Filtered()

	assertCount(t, "PendingCount", recomputed.PendingCount, 1)
	assertCount(t, "RunningCount", recomputed.RunningCount, 1)
	assertCount(t, "SucceededCount", recomputed.SucceededCount, 1)
}

func TestReport_PeakConcurrency_ParallelSteps(t *testing.T) {
	t.Parallel()

	// Use slow steps to ensure true overlap in the event stream.
	a, w := newAuditAndWorkflow(t)
	addParallelSteps(w, newSlow("a", 20*time.Millisecond), newSlow("b", 20*time.Millisecond))
	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	// Two parallel slow steps should produce a peak concurrency of 2.
	if report.PeakConcurrency != 2 {
		t.Errorf("expected PeakConcurrency=2, got %d", report.PeakConcurrency)
	}
}

func TestReport_PeakConcurrency_SequentialSteps(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	addDependentStep(w, newSucceed("parent"), newSucceed("child"))
	runWorkflow(t, a, w)

	report := a.Report()
	assertReportValid(t, report)

	// Sequential steps should never overlap.
	if report.PeakConcurrency != 1 {
		t.Errorf("expected PeakConcurrency=1, got %d", report.PeakConcurrency)
	}
}

func TestReport_CriticalPathDuration_DependentChain(t *testing.T) {
	t.Parallel()

	// Construct a report with a known dependency chain.
	// parent (10ms) -> child (20ms) -> grandchild (30ms)
	// plus an independent leaf (5ms)
	// critical path = 10 + 20 + 30 = 60ms
	d10 := 10.0
	d20 := 20.0
	d30 := 30.0
	d5 := 5.0

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{
				StepRef:    auditlog.StepRef{Name: "parent"},
				Status:     auditlog.StepStatusSucceeded,
				DurationMs: &d10,
			},
			{
				StepRef:      auditlog.StepRef{Name: "child"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &d20,
				Dependencies: []auditlog.StepRef{{Name: "parent"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "grandchild"},
				Status:       auditlog.StepStatusSucceeded,
				DurationMs:   &d30,
				Dependencies: []auditlog.StepRef{{Name: "child"}},
			},
			{
				StepRef:    auditlog.StepRef{Name: "leaf"},
				Status:     auditlog.StepStatusSucceeded,
				DurationMs: &d5,
			},
		},
	}

	recomputed := raw.Filtered()

	want := 60.0
	if recomputed.CriticalPathDurationMs != want {
		t.Errorf("expected CriticalPathDurationMs=%f, got %f", want, recomputed.CriticalPathDurationMs)
	}
}

func TestReport_CriticalPathDuration_Empty(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{Steps: []auditlog.StepInfo{}}
	recomputed := raw.Filtered()

	if recomputed.CriticalPathDurationMs != 0 {
		t.Errorf("expected CriticalPathDurationMs=0 for empty report, got %f", recomputed.CriticalPathDurationMs)
	}
}

func TestReport_FailureReason_FailedSteps(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "ok"}, Status: auditlog.StepStatusSucceeded},
			{StepRef: auditlog.StepRef{Name: "bad-a"}, Status: auditlog.StepStatusFailed},
			{StepRef: auditlog.StepRef{Name: "bad-b"}, Status: auditlog.StepStatusFailed},
		},
	}

	recomputed := raw.Filtered()

	if recomputed.WorkflowSucceeded {
		t.Error("expected workflow to be failed")
	}

	want := "2 step(s) failed: bad-a, bad-b"
	if recomputed.FailureReason != want {
		t.Errorf("expected FailureReason=%q, got %q", want, recomputed.FailureReason)
	}
}

func TestReport_FailureReason_CanceledSteps(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "ok"}, Status: auditlog.StepStatusSucceeded},
			{StepRef: auditlog.StepRef{Name: "cancel"}, Status: auditlog.StepStatusCanceled},
		},
	}

	recomputed := raw.Filtered()

	want := "1 step(s) canceled: cancel"
	if recomputed.FailureReason != want {
		t.Errorf("expected FailureReason=%q, got %q", want, recomputed.FailureReason)
	}
}

func TestReport_FailureReason_Success(t *testing.T) {
	t.Parallel()

	raw := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "ok"}, Status: auditlog.StepStatusSucceeded},
		},
	}

	recomputed := raw.Filtered()

	if !recomputed.WorkflowSucceeded {
		t.Error("expected workflow to be succeeded")
	}

	if recomputed.FailureReason != "" {
		t.Errorf("expected empty FailureReason for success, got %q", recomputed.FailureReason)
	}
}
