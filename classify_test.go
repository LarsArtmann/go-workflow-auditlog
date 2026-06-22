package auditlog_test

import (
	"errors"
	"fmt"
	"testing"

	errorfamily "github.com/larsartmann/go-error-family"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// allPublicSentinels returns every exported auditlog sentinel error. Shared by
// tests that need to iterate the full set (errors.Is identity, classification
// membership) without duplicating the list.
func allPublicSentinels() []error {
	return []error{
		auditlog.ErrEventCountMismatch,
		auditlog.ErrStepCountMismatch,
		auditlog.ErrStatusDrift,
		auditlog.ErrCountMismatch,
		auditlog.ErrEmpty,
		auditlog.ErrNoEvents,
		auditlog.ErrOversizedLine,
		auditlog.ErrWorkflowIDPathSep,
		auditlog.ErrReplayNoEvents,
		auditlog.ErrReportLoadFailed,
		auditlog.ErrRenderFailed,
		auditlog.ErrExportWriteFailed,
	}
}

func TestClassify_PublicSentinels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		family    errorfamily.Family
		exitCode  int
		retryable bool
	}{
		// Corruption — data integrity violations (exit 65, not retryable)
		{
			name:      "ErrEventCountMismatch is Corruption",
			err:       auditlog.ErrEventCountMismatch,
			family:    errorfamily.Corruption,
			exitCode:  65,
			retryable: false,
		},
		{
			name:      "ErrStepCountMismatch is Corruption",
			err:       auditlog.ErrStepCountMismatch,
			family:    errorfamily.Corruption,
			exitCode:  65,
			retryable: false,
		},
		{
			name:      "ErrStatusDrift is Corruption",
			err:       auditlog.ErrStatusDrift,
			family:    errorfamily.Corruption,
			exitCode:  65,
			retryable: false,
		},
		{
			name:      "ErrCountMismatch is Corruption",
			err:       auditlog.ErrCountMismatch,
			family:    errorfamily.Corruption,
			exitCode:  65,
			retryable: false,
		},
		// Rejection — bad caller input (exit 1, not retryable)
		{
			name:      "ErrEmpty is Rejection",
			err:       auditlog.ErrEmpty,
			family:    errorfamily.Rejection,
			exitCode:  1,
			retryable: false,
		},
		{
			name:      "ErrNoEvents is Rejection",
			err:       auditlog.ErrNoEvents,
			family:    errorfamily.Rejection,
			exitCode:  1,
			retryable: false,
		},
		{
			name:      "ErrOversizedLine is Rejection",
			err:       auditlog.ErrOversizedLine,
			family:    errorfamily.Rejection,
			exitCode:  1,
			retryable: false,
		},
		{
			name:      "ErrWorkflowIDPathSep is Rejection",
			err:       auditlog.ErrWorkflowIDPathSep,
			family:    errorfamily.Rejection,
			exitCode:  1,
			retryable: false,
		},
		{
			name:      "ErrReplayNoEvents is Rejection",
			err:       auditlog.ErrReplayNoEvents,
			family:    errorfamily.Rejection,
			exitCode:  1,
			retryable: false,
		},
		// Transient — retryable (exit 75)
		{
			name:      "ErrReportLoadFailed is Transient",
			err:       auditlog.ErrReportLoadFailed,
			family:    errorfamily.Transient,
			exitCode:  75,
			retryable: true,
		},
		// Infrastructure — system-level, not retryable (exit 69)
		{
			name:      "ErrRenderFailed is Infrastructure",
			err:       auditlog.ErrRenderFailed,
			family:    errorfamily.Infrastructure,
			exitCode:  69,
			retryable: false,
		},
		{
			name:      "ErrExportWriteFailed is Infrastructure",
			err:       auditlog.ErrExportWriteFailed,
			family:    errorfamily.Infrastructure,
			exitCode:  69,
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotFamily := errorfamily.Classify(tt.err)
			if gotFamily != tt.family {
				t.Errorf("Classify(%v) = %v, want %v", tt.err, gotFamily, tt.family)
			}

			gotExit := errorfamily.ExitCode(tt.err)
			if gotExit != tt.exitCode {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, gotExit, tt.exitCode)
			}

			gotRetryable := errorfamily.IsRetryable(tt.err)
			if gotRetryable != tt.retryable {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, gotRetryable, tt.retryable)
			}
		})
	}
}

func TestClassify_WrappedErrorPreservesClassification(t *testing.T) {
	t.Parallel()

	// auditlog wraps sentinels via fmt.Errorf("%w: ...", sentinel, ...).
	// errorfamily.Classify must unwrap the chain and find the registered sentinel.
	wrapped := fmt.Errorf("%w: got %d, want %d", auditlog.ErrEventCountMismatch, 5, 3)

	if got := errorfamily.Classify(wrapped); got != errorfamily.Corruption {
		t.Errorf("Classify(wrapped ErrEventCountMismatch) = %v, want Corruption", got)
	}

	if got := errorfamily.ExitCode(wrapped); got != 65 {
		t.Errorf("ExitCode(wrapped ErrEventCountMismatch) = %d, want 65", got)
	}

	// errors.Is still works on the wrapped error — registration doesn't break identity.
	if !errors.Is(wrapped, auditlog.ErrEventCountMismatch) {
		t.Error("errors.Is(wrapped, ErrEventCountMismatch) = false, want true")
	}
}

func TestClassify_WrappedIOErrorPreservesClassification(t *testing.T) {
	t.Parallel()

	// Real-world wrapping pattern used in loader.go, plugin.go, etc.
	loadErr := fmt.Errorf("%w: open %q: %w", auditlog.ErrReportLoadFailed, "report.json", errors.New("no such file"))

	if got := errorfamily.Classify(loadErr); got != errorfamily.Transient {
		t.Errorf("Classify(wrapped ErrReportLoadFailed) = %v, want Transient", got)
	}

	if !errorfamily.IsRetryable(loadErr) {
		t.Error("IsRetryable(wrapped ErrReportLoadFailed) = false, want true")
	}

	if !errors.Is(loadErr, auditlog.ErrReportLoadFailed) {
		t.Error("errors.Is(wrapped, ErrReportLoadFailed) = false, want true")
	}

	renderErr := fmt.Errorf("%w: render d2 diagram: %w", auditlog.ErrRenderFailed, errors.New("bad node"))

	if got := errorfamily.Classify(renderErr); got != errorfamily.Infrastructure {
		t.Errorf("Classify(wrapped ErrRenderFailed) = %v, want Infrastructure", got)
	}

	if !errors.Is(renderErr, auditlog.ErrRenderFailed) {
		t.Error("errors.Is(wrapped, ErrRenderFailed) = false, want true")
	}

	writeErr := fmt.Errorf("%w: flush temp file: %w", auditlog.ErrExportWriteFailed, errors.New("disk full"))

	if got := errorfamily.Classify(writeErr); got != errorfamily.Infrastructure {
		t.Errorf("Classify(wrapped ErrExportWriteFailed) = %v, want Infrastructure", got)
	}

	if !errors.Is(writeErr, auditlog.ErrExportWriteFailed) {
		t.Error("errors.Is(wrapped, ErrExportWriteFailed) = false, want true")
	}
}

func TestClassify_NestedWrapping(t *testing.T) {
	t.Parallel()

	// Double-wrap to prove multi-level unwrap works.
	inner := fmt.Errorf("%w (max %d bytes)", auditlog.ErrOversizedLine, 1<<20)
	outer := fmt.Errorf("read line 42: %w", inner)

	if got := errorfamily.Classify(outer); got != errorfamily.Rejection {
		t.Errorf("Classify(double-wrapped ErrOversizedLine) = %v, want Rejection", got)
	}
}

func TestClassify_UnregisteredErrorDefaultsToTransient(t *testing.T) {
	t.Parallel()

	// errorfamily.Classify defaults unknown errors to Transient (fail-open for retry).
	// A plain error with no sentinel in the chain should classify as Transient.
	plain := errors.New("something went wrong")

	if got := errorfamily.Classify(plain); got != errorfamily.Transient {
		t.Errorf("Classify(unregistered error) = %v, want Transient (default fail-open)", got)
	}
}

func TestClassify_ErrorsIsUnchanged(t *testing.T) {
	t.Parallel()

	// Registration must not alter errors.Is behavior — this is the core
	// guarantee of Strategy A (registration, not replacement).
	for _, s := range allPublicSentinels() {
		if !errors.Is(s, s) {
			t.Errorf("errors.Is(%v, %v) = false, want true (identity must hold)", s, s)
		}
	}
}

func TestRegisterClassifications_CustomRegistry(t *testing.T) {
	t.Parallel()

	reg := errorfamily.NewRegistry()
	auditlog.RegisterClassifications(reg)

	if got := reg.Classify(auditlog.ErrStatusDrift); got != errorfamily.Corruption {
		t.Errorf("custom registry Classify(ErrStatusDrift) = %v, want Corruption", got)
	}

	if got := reg.Classify(auditlog.ErrEmpty); got != errorfamily.Rejection {
		t.Errorf("custom registry Classify(ErrEmpty) = %v, want Rejection", got)
	}

	if got := reg.Classify(auditlog.ErrReportLoadFailed); got != errorfamily.Transient {
		t.Errorf("custom registry Classify(ErrReportLoadFailed) = %v, want Transient", got)
	}

	if got := reg.Classify(auditlog.ErrRenderFailed); got != errorfamily.Infrastructure {
		t.Errorf("custom registry Classify(ErrRenderFailed) = %v, want Infrastructure", got)
	}
}

func TestErrorClassifications_ContainsAllPublicSentinels(t *testing.T) {
	t.Parallel()

	classifications := auditlog.ErrorClassifications()

	for _, sentinel := range allPublicSentinels() {
		family, ok := classifications[sentinel]
		if !ok {
			t.Errorf("ErrorClassifications() missing sentinel %v", sentinel)

			continue
		}

		if !family.IsValid() {
			t.Errorf("ErrorClassifications()[%v] = %v, want a valid Family", sentinel, family)
		}
	}
}
