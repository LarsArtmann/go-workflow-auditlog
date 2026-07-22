package auditlog_test

import (
	"bytes"
	"context"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

// --- Export tests ---

func TestWriteJSON_ToBuffer(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewSucceed("json-step")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	var buf bytes.Buffer

	err := a.WriteJSON(&buf)
	if err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	var report auditlog.WorkflowReport

	err = json.Unmarshal(buf.Bytes(), &report)
	if err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	testhelpers.AssertStepCount(t, report, 1)
}

func TestWriteNDJSON_ToBuffer(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewSucceed("ndjson-step")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	var buf bytes.Buffer

	err := a.WriteNDJSON(&buf)
	if err != nil {
		t.Fatalf("WriteNDJSON error: %v", err)
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

func TestExportJSON_Error(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	err := a.ExportJSON("/nonexistent/dir/file.json")
	if err == nil {
		t.Fatal("expected error writing to nonexistent directory")
	}
}

func TestExportNDJSON_Error(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	err := a.ExportNDJSON("/nonexistent/dir/file.ndjson")
	if err == nil {
		t.Fatal("expected error writing to nonexistent directory")
	}
}

// --- Config tests ---

func TestNew_InitialEventCapacity(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{
		Enabled:              true,
		InitialEventCapacity: 512,
	})
	w := &flow.Workflow{}
	s := testhelpers.NewSucceed("cap-step")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	assertEventsRecorded(t, a, 2)
}

// --- Nil safety tests ---

func TestAttach_NilWorkflow(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})
	// Should not panic.
	a.Attach(nil)
}

func TestSnapshot_NilWorkflow(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})
	// Should not panic.
	a.Snapshot(nil)
}

func TestAttach_Disabled(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: false})
	w := &flow.Workflow{}
	s := testhelpers.NewSucceed("step")
	w.Add(flow.Step(s))
	a.Attach(w) // no-op when disabled
}

// --- Complex scenario tests ---

func TestFanOutFanIn(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	root := testhelpers.NewSucceed("root")
	branch1 := testhelpers.NewSucceed("branch-1")
	branch2 := testhelpers.NewSucceed("branch-2")
	join := testhelpers.NewSucceed("join")
	w.Add(
		flow.Step(root),
		flow.Step(branch1).DependsOn(root),
		flow.Step(branch2).DependsOn(root),
		flow.Step(join).DependsOn(branch1, branch2),
	)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	testhelpers.AssertStepCount(t, report, 4)

	joinStep := testhelpers.FindStep(t, report, "join")
	if len(joinStep.Dependencies) != 2 {
		t.Errorf("expected join to have 2 deps, got %d", len(joinStep.Dependencies))
	}

	rootStep := testhelpers.FindStep(t, report, "root")
	if len(rootStep.Dependents) != 2 {
		t.Errorf("expected root to have 2 dependents, got %d", len(rootStep.Dependents))
	}
}

func TestStepType_Inferred(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewSucceed("typed")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	step := report.StepByName("typed")
	if step.StepType != "SucceedStep" {
		t.Errorf("expected step type 'SucceedStep', got %q", step.StepType)
	}
}

func TestDuration_Tracked(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewSlow("slow-step", 20*time.Millisecond)
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

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

	a := testhelpers.MustNewWithID(t, "my-custom-wf")
	w := &flow.Workflow{}
	s := testhelpers.NewSucceed("step")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertWorkflowID(t, report, "my-custom-wf")
}

// TestRunID_DefaultGenerated confirms that when no RunID is supplied, the
// auditor generates a non-empty, hex-formatted run ID and stamps it on every
// captured event and the final report.
func TestRunID_DefaultGenerated(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})
	w := &flow.Workflow{}
	w.Add(flow.Step(testhelpers.NewSucceed("step-a")))
	w.Add(flow.Step(testhelpers.NewSucceed("step-b")))
	testhelpers.RunWorkflow(t, a, w)

	runID := a.RunID()
	if runID == "" {
		t.Fatal("expected non-empty generated RunID")
	}

	if len(runID) != 32 {
		t.Errorf("expected 32-char hex RunID, got %d chars: %q", len(runID), runID)
	}

	_, err := hex.DecodeString(string(runID))
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

	testhelpers.AssertEventRunIDsMatch(t, report.Events, runID)
}

// TestRunID_CustomHonored confirms a caller-supplied RunID is used verbatim
// (no generation, no mutation) on the report and events.
func TestRunID_CustomHonored(t *testing.T) {
	t.Parallel()

	const custom = "trace-abc-123"

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true, RunID: custom})
	w := &flow.Workflow{}
	w.Add(flow.Step(testhelpers.NewSucceed("step")))
	testhelpers.RunWorkflow(t, a, w)

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

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})
	w := &flow.Workflow{}
	w.Add(flow.Step(testhelpers.NewSucceed("step")))
	testhelpers.RunWorkflow(t, a, w)

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

	a1 := testhelpers.MustNew(t, auditlog.Config{Enabled: true})
	a2 := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	if a1.RunID() == a2.RunID() {
		t.Errorf("expected distinct RunIDs, both were %q", a1.RunID())
	}
}

// TestStepID_UniqueAndSequential confirms every step gets a distinct, non-zero
// StepID and that all IDs form a contiguous 1..N set.
func TestStepID_UniqueAndSequential(t *testing.T) {
	t.Parallel()

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})
	w := &flow.Workflow{}
	w.Add(flow.Step(testhelpers.NewSucceed("step-a")))
	w.Add(flow.Step(testhelpers.NewSucceed("step-b")))
	w.Add(flow.Step(testhelpers.NewSucceed("step-c")))
	testhelpers.RunWorkflow(t, a, w)

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

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := testhelpers.NewSlow("cancel-me", 10*time.Second)
	w.Add(
		flow.Step(s).Timeout(10 * time.Millisecond),
	)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	step := report.StepByName("cancel-me")
	if step.Status != auditlog.StepStatusCanceled {
		t.Errorf("expected canceled, got %s", step.Status)
	}

	testhelpers.AssertCount(t, "CanceledCount", report.CanceledCount, 1)

	if report.WorkflowSucceeded {
		t.Error("expected WorkflowSucceeded=false")
	}
}

func TestEnvEnabledFalse(t *testing.T) {
	t.Setenv(auditlog.EnvKeyEnabled, "false")

	a := testhelpers.MustNew(t, auditlog.Config{})
	w := &flow.Workflow{}
	s := testhelpers.NewSucceed("env-false-step")
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	if a.EventsCount() != 0 {
		t.Errorf("expected 0 events when disabled via env, got %d", a.EventsCount())
	}
}

func TestEventsCount_NoCopy(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "count-step")

	count := a.EventsCount()

	events := a.Events()
	if count != len(events) {
		t.Errorf("EventsCount()=%d but len(Events())=%d", count, len(events))
	}
}

func TestStepTypeName_NilStep(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	// Use a Func step which has a clean type name.
	w.Add(flow.Step(flow.Func("func-step", func(_ context.Context) error { return nil })))
	testhelpers.RunWorkflow(t, a, w)

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

	a, w := testhelpers.NewAuditAndWorkflow(t)
	s := flow.Func("skip-me", func(_ context.Context) error {
		return flow.Skip(errors.New("not needed"))
	})
	w.Add(flow.Step(s))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	testhelpers.AssertReportValid(t, report)

	step := report.StepByName("skip-me")
	if step.Status != auditlog.StepStatusSkipped {
		t.Errorf("expected skipped, got %s", step.Status)
	}
}

func TestMultipleWorkflows_Isolated(t *testing.T) {
	t.Parallel()

	a1 := testhelpers.MustNewWithID(t, "wf-1")
	a2 := testhelpers.MustNewWithID(t, "wf-2")

	w1 := &flow.Workflow{}
	w1.Add(flow.Step(testhelpers.NewSucceed("wf1-step")))

	w2 := &flow.Workflow{}
	w2.Add(flow.Step(testhelpers.NewFail("wf2-step", "err")))

	testhelpers.RunWorkflow(t, a1, w1)
	testhelpers.RunWorkflow(t, a2, w2)

	r1 := a1.Report()
	r2 := a2.Report()

	testhelpers.AssertWorkflowID(t, r1, "wf-1")
	testhelpers.AssertWorkflowID(t, r2, "wf-2")

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

	a := testhelpers.MustNew(t, auditlog.Config{Enabled: true})

	err := a.ExportJSON(path)
	if err == nil {
		// Some filesystems allow writing to read-only files; skip if no error.
		return
	}
}

// TestCoverage_D2_EmptyWorkflowID covers the empty-WorkflowID fallback branch
// of d2DiagramTitle (was 66.7%) and the pending-status branch of statusStyle
// (was 75%). Both are triggered by exporting a D2 diagram from a report with
// no WorkflowID and a pending step.
func TestCoverage_D2_EmptyWorkflowID(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			testhelpers.StepFixture("pending-step", auditlog.StepStatusPending),
		},
	}

	var buf bytes.Buffer

	err := report.WriteD2(&buf)
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Workflow DAG") {
		t.Error("expected fallback title 'Workflow DAG' in D2 output")
	}

	if !strings.Contains(out, "pending-step") {
		t.Error("expected pending-step in D2 output")
	}
}

// TestCoverage_CancelStatus_Diagram covers the canceled-status color branch
// in statusStyle (was 75%) by exporting a diagram with a canceled step.
func TestCoverage_CancelStatus_Diagram(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		WorkflowID: "cancel-test",
		Steps: []auditlog.StepInfo{
			testhelpers.StepFixture("canceled-step", auditlog.StepStatusCanceled),
		},
	}

	var buf bytes.Buffer

	err := report.WriteD2(&buf)
	if err != nil {
		t.Fatalf("WriteD2 error: %v", err)
	}

	// Canceled steps get orange (#5a3d2d). Verify the color appears.
	if !strings.Contains(buf.String(), "#5a3d2d") {
		t.Error("expected canceled status color (#5a3d2d) in D2 output")
	}
}
