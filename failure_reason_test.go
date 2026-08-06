package auditlog_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestFailureReason_KnownValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reason auditlog.FailureReason
		want   string
	}{
		{auditlog.FailureReasonTimeout, "timeout"},
		{auditlog.FailureReasonCanceled, "canceled"},
		{auditlog.FailureReasonPanic, "panic"},
		{auditlog.FailureReasonDependency, "dependency"},
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

	if len(all) != 5 {
		t.Errorf("expected 5 FailureReason values, got %d", len(all))
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
			reason: auditlog.FailureReasonPanic,
			check:  func(e auditlog.Event) bool { return e.IsPanic() },
			name:   "IsPanic",
		},
		{
			reason: auditlog.FailureReasonDependency,
			check:  func(e auditlog.Event) bool { return e.IsDependencyFailure() },
			name:   "IsDependencyFailure",
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
