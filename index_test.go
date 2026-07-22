package auditlog_test

import (
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

// TestReportIndex_AgreesWithLinearQueries confirms the O(1) ReportIndex
// returns the same results as the O(n) report query methods.
func TestReportIndex_AgreesWithLinearQueries(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	save := testhelpers.NewSucceed("save")
	testhelpers.AddDependentStep(w, fetch, save)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	index := auditlog.NewReportIndex(report)

	// StepByName agrees.
	got := index.StepByName("fetch")
	if got == nil || got.Name != "fetch" {
		t.Errorf("StepByName(fetch) = %+v, want fetch step", got)
	}

	testhelpers.AssertNilStep(t, index.StepByName("nonexistent"), "expected nil for unknown step name")

	// StepByID agrees with the StepID assigned in the report.
	for _, step := range report.Steps {
		if byID := index.StepByID(step.StepID); byID == nil || byID.Name != step.Name {
			t.Errorf("StepByID(%d) mismatch: got %+v, want %q", step.StepID, byID, step.Name)
		}
	}

	testhelpers.AssertNilStep(t, index.StepByID(99999), "expected nil for unknown step ID")

	// EventsByStep agrees with the linear method.
	for _, name := range []string{"fetch", "save"} {
		idxEvents := index.EventsByStep(name)
		repEvents := report.EventsByStep(name)

		if len(idxEvents) != len(repEvents) {
			t.Errorf("EventsByStep(%s): index has %d, report has %d",
				name, len(idxEvents), len(repEvents))
		}
	}

	// EventsByType agrees.
	for _, et := range []auditlog.EventType{
		auditlog.EventTypeAttemptStart,
		auditlog.EventTypeAttemptEnd,
	} {
		if len(index.EventsByType(et)) != len(report.EventsByType(et)) {
			t.Errorf("EventsByType(%s): index/report mismatch", et)
		}
	}
}

// TestReportIndex_EmptyReport confirms an index over an empty report returns
// nil/zero without panicking.
func TestReportIndex_EmptyReport(t *testing.T) {
	t.Parallel()

	index := auditlog.NewReportIndex(auditlog.WorkflowReport{})

	testhelpers.AssertNilStep(t, index.StepByName("anything"), "expected nil StepByName on empty report")

	testhelpers.AssertNilStep(t, index.StepByID(1), "expected nil StepByID on empty report")

	if events := index.EventsByStep("anything"); events != nil {
		t.Errorf("expected nil EventsByStep on empty report, got %d", len(events))
	}
}
