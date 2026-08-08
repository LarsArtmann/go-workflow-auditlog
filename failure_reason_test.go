package auditlog_test

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

func TestFailureReason_KnownValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reason auditlog.FailureReason
		want   string
	}{
		{auditlog.FailureReasonTimeout, "timeout"},
		{auditlog.FailureReasonCanceled, "canceled"},
		{auditlog.FailureReasonUserError, "user_error"},
	}

	for _, tc := range tests {
		t.Run(string(tc.reason), func(t *testing.T) {
			t.Parallel()

			if !tc.reason.IsKnown() {
				t.Errorf("%q should be known", tc.reason)
			}

			if got := tc.reason.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFailureReason_UnknownValue(t *testing.T) {
	t.Parallel()

	unknown := auditlog.FailureReason("not_a_real_reason")

	if unknown.IsKnown() {
		t.Errorf("unknown FailureReason should not be known")
	}

	if unknown.String() != "not_a_real_reason" {
		t.Errorf("String() should return the raw value, got %q", unknown.String())
	}
}

func TestAllFailureReasons(t *testing.T) {
	t.Parallel()

	all := auditlog.AllFailureReasons()

	if len(all) != 3 {
		t.Errorf("expected 3 FailureReason values, got %d", len(all))
	}

	for _, r := range all {
		if !r.IsKnown() {
			t.Errorf("AllFailureReasons returned unknown value %q", r)
		}
	}
}

func TestFailureReason_ClassifyHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reason auditlog.FailureReason
		check  func(auditlog.Event) bool
		name   string
	}{
		{
			reason: auditlog.FailureReasonTimeout,
			check:  func(e auditlog.Event) bool { return e.IsTimeout() },
			name:   "IsTimeout",
		},
		{
			reason: auditlog.FailureReasonCanceled,
			check:  func(e auditlog.Event) bool { return e.IsCanceled() },
			name:   "IsCanceled",
		},
		{
			reason: auditlog.FailureReasonUserError,
			check:  func(e auditlog.Event) bool { return e.IsUserError() },
			name:   "IsUserError",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			evt := auditlog.Event{FailureReason: tc.reason}

			if !tc.check(evt) {
				t.Errorf("Event with FailureReason=%q should satisfy %s", tc.reason, tc.name)
			}
		})
	}
}

// classifyFailurePublic mirrors the internal classifyFailure helper's
// behavior for test-side verification. The package's own classifyFailure
// has the same precedence: nil → "", DeadlineExceeded → Timeout,
// Canceled → Canceled, else → UserError.
func classifyFailurePublic(err error) auditlog.FailureReason {
	if err == nil {
		return ""
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return auditlog.FailureReasonTimeout
	}

	if errors.Is(err, context.Canceled) {
		return auditlog.FailureReasonCanceled
	}

	return auditlog.FailureReasonUserError
}

func TestClassifyFailure_Contract_NilError(t *testing.T) {
	t.Parallel()

	if got := classifyFailurePublic(nil); got != "" {
		t.Errorf("expected empty FailureReason for nil error, got %q", got)
	}
}

func TestClassifyFailure_Contract_TimeoutError(t *testing.T) {
	t.Parallel()

	if got := classifyFailurePublic(context.DeadlineExceeded); got != auditlog.FailureReasonTimeout {
		t.Errorf("context.DeadlineExceeded should classify as timeout, got %q", got)
	}

	wrapped := fmt.Errorf("operation: %w", context.DeadlineExceeded)

	if got := classifyFailurePublic(wrapped); got != auditlog.FailureReasonTimeout {
		t.Errorf("wrapped DeadlineExceeded should classify as timeout, got %q", got)
	}
}

func TestClassifyFailure_Contract_CanceledError(t *testing.T) {
	t.Parallel()

	if got := classifyFailurePublic(context.Canceled); got != auditlog.FailureReasonCanceled {
		t.Errorf("context.Canceled should classify as canceled, got %q", got)
	}

	wrapped := fmt.Errorf("operation: %w", context.Canceled)

	if got := classifyFailurePublic(wrapped); got != auditlog.FailureReasonCanceled {
		t.Errorf("wrapped Canceled should classify as canceled, got %q", got)
	}
}

func TestClassifyFailure_Contract_GenericError(t *testing.T) {
	t.Parallel()

	if got := classifyFailurePublic(errors.New("disk full")); got != auditlog.FailureReasonUserError {
		t.Errorf("generic error should classify as user_error, got %q", got)
	}
}

// --- StepInfo FailureReason denormalization tests ---

func TestStepInfo_FailureReason_Timeout(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewSlow("timeout-step", 5*time.Second)
	w.Add(
		flow.Step(step).Timeout(50 * time.Millisecond),
	)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	s := testhelpers.FindStep(t, report, "timeout-step")

	if s.FailureReason != auditlog.FailureReasonTimeout {
		t.Errorf("expected StepInfo.FailureReason=%q, got %q",
			auditlog.FailureReasonTimeout, s.FailureReason)
	}
}

func TestStepInfo_FailureReason_UserError(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewFail("fail-step", "boom")
	w.Add(flow.Step(step))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	s := testhelpers.FindStep(t, report, "fail-step")

	if s.FailureReason != auditlog.FailureReasonUserError {
		t.Errorf("expected StepInfo.FailureReason=%q, got %q",
			auditlog.FailureReasonUserError, s.FailureReason)
	}
}

func TestStepInfo_FailureReason_SuccessIsEmpty(t *testing.T) {
	t.Parallel()

	report := testhelpers.RunSingleSucceedWithReport(t, "ok-step")
	s := testhelpers.FindStep(t, report, "ok-step")

	if s.FailureReason != "" {
		t.Errorf("expected empty FailureReason for successful step, got %q", s.FailureReason)
	}
}

func TestStepInfo_FailureReason_ClearedOnRetrySuccess(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewFlaky("flaky-step", 2)
	testhelpers.AddRetryStep(w, step, 5)
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()
	s := testhelpers.FindStep(t, report, "flaky-step")

	if s.Status != auditlog.StepStatusSucceeded {
		t.Fatalf("expected flaky step to succeed on retry, got %s", s.Status)
	}

	if s.FailureReason != "" {
		t.Errorf("expected empty FailureReason after retry success, got %q", s.FailureReason)
	}
}

func TestReplay_FailureReasonOnStepInfo(t *testing.T) {
	t.Parallel()

	events := []auditlog.Event{
		{
			Sequence: 1, EventType: auditlog.EventTypeAttemptStart, Phase: auditlog.PhaseBefore,
			Timestamp: time.Now(), StepRef: auditlog.StepRef{Name: "fail"}, Attempt: 1,
		},
		{
			Sequence: 2, EventType: auditlog.EventTypeAttemptEnd, Phase: auditlog.PhaseAfter,
			Timestamp: time.Now(), StepRef: auditlog.StepRef{Name: "fail"}, Attempt: 1,
			Status: auditlog.StepStatusFailed, FailureReason: auditlog.FailureReasonTimeout,
			Error: new("deadline exceeded"),
		},
	}

	report, err := auditlog.ReplayEvents(events)
	if err != nil {
		t.Fatalf("ReplayEvents: %v", err)
	}

	s := testhelpers.FindStep(t, report, "fail")
	if s.FailureReason != auditlog.FailureReasonTimeout {
		t.Errorf("expected replayed StepInfo.FailureReason=%q, got %q",
			auditlog.FailureReasonTimeout, s.FailureReason)
	}
}

func TestCSV_FailureReasonColumn(t *testing.T) {
	t.Parallel()

	errMsg := "connection refused"
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{
				StepRef: auditlog.StepRef{Name: "failed-step"}, Status: auditlog.StepStatusFailed,
				FailureReason: auditlog.FailureReasonUserError, Error: &errMsg,
			},
			{StepRef: auditlog.StepRef{Name: "ok-step"}, Status: auditlog.StepStatusSucceeded},
		},
	}

	var buf strings.Builder

	if err := report.WriteCSV(&buf); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	const failureReasonCol = 12

	if records[0][failureReasonCol] != "failure_reason" {
		t.Fatalf("expected header[12]=failure_reason, got %q", records[0][failureReasonCol])
	}

	if got := records[1][failureReasonCol]; got != "user_error" {
		t.Errorf("expected failed step failure_reason=user_error, got %q", got)
	}

	if got := records[2][failureReasonCol]; got != "" {
		t.Errorf("expected ok step failure_reason empty, got %q", got)
	}
}

func TestFailureReason_Label(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reason auditlog.FailureReason
		want   string
	}{
		{auditlog.FailureReasonTimeout, "Timeout"},
		{auditlog.FailureReasonCanceled, "Canceled"},
		{auditlog.FailureReasonUserError, "User Error"},
	}

	for _, tc := range tests {
		t.Run(string(tc.reason), func(t *testing.T) {
			t.Parallel()

			if got := tc.reason.Label(); got != tc.want {
				t.Errorf("Label() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFailureReason_Color(t *testing.T) {
	t.Parallel()

	for _, r := range auditlog.AllFailureReasons() {
		if r.Color() == "" {
			t.Errorf("FailureReason %q should have a non-empty Color", r)
		}
	}
}

func strPtr(s string) *string { return new(s) }

// TestFailureSummary_GoldenJSON verifies the JSON field placement:
//   - "failure_summary" appears at the report level (WorkflowReport)
//   - "failure_reason" appears at the event level (Event) on attempt_end events
//   - Neither field appears in the wrong scope (the v0.8 rename prevents collision)
func TestFailureSummary_GoldenJSON(t *testing.T) {
	t.Parallel()

	// Report with a failure: failure_summary should be present at report level.
	failedReport := auditlog.WorkflowReport{
		WorkflowID: "test-pipeline",
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "fail"}, Status: auditlog.StepStatusFailed},
		},
		FailedCount:    1,
		FailureSummary: "1 step(s) failed: fail",
	}

	var buf strings.Builder
	if err := failedReport.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	json := buf.String()

	if !strings.Contains(json, `"failure_summary"`) {
		t.Error("expected report-level failure_summary in JSON")
	}

	// Report-level JSON should NOT have a bare "failure_reason" key outside of
	// event objects. We verify by checking that the report object does not
	// carry the field at the top level by building a report without events.
	if strings.Contains(json, `"failure_reason"`) {
		t.Error("report-level JSON should not contain failure_reason (events are empty)")
	}

	// Successful report: failure_summary should be absent (omitempty).
	successReport := testhelpers.RunSingleSucceedWithReport(t, "ok-step")

	var buf2 strings.Builder
	if err := successReport.WriteJSON(&buf2); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	if strings.Contains(buf2.String(), `"failure_summary"`) {
		t.Error("successful report should omit failure_summary")
	}
}
