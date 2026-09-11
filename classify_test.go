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
		auditlog.ErrMigrationEmptyInput,
		auditlog.ErrMigrationMissingVersion,
		auditlog.ErrFileExists,
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
		{
			name:      "ErrMigrationEmptyInput is Rejection",
			err:       auditlog.ErrMigrationEmptyInput,
			family:    errorfamily.Rejection,
			exitCode:  1,
			retryable: false,
		},
		{
			name:      "ErrMigrationMissingVersion is Rejection",
			err:       auditlog.ErrMigrationMissingVersion,
			family:    errorfamily.Rejection,
			exitCode:  1,
			retryable: false,
		},
		{
			name:      "ErrFileExists is Rejection",
			err:       auditlog.ErrFileExists,
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

func TestClassify_IntrinsicClassificationWithoutRegistry(t *testing.T) {
	t.Parallel()

	// Owned sentinels are *errorfamily.Error values carrying their family
	// intrinsically: a registry with NO registrations must classify them via
	// the Classified interface alone. The re-exported go-ndjson sentinels are
	// third-party errors — on an empty registry they fall through to the
	// Transient default, which is exactly why init() registers them.
	reg := errorfamily.NewRegistry()

	unregistered := 0

	for sentinel, want := range auditlog.ErrorClassifications() {
		typed, owned := errors.AsType[*errorfamily.Error](sentinel)
		if !owned {
			unregistered++

			continue
		}

		t.Run(typed.Code(), func(t *testing.T) {
			t.Parallel()

			if got := reg.Classify(sentinel); got != want {
				t.Errorf("intrinsic Classify(%s) = %v, want %v", typed.Code(), got, want)
			}
		})
	}

	// Exactly the three go-ndjson re-exports rely on the registry channel.
	if unregistered != 3 {
		t.Errorf("ErrorClassifications() has %d non-intrinsic sentinels, want 3 (ErrEmpty, ErrNoEvents, ErrOversizedLine)", unregistered)
	}

	if got := reg.Classify(auditlog.ErrEmpty); got != errorfamily.Transient {
		t.Errorf("Classify(ErrEmpty) on empty registry = %v, want Transient (unregistered default)", got)
	}
}

func TestErrorClassifications_CodesUniquePerFamily(t *testing.T) {
	t.Parallel()

	// *errorfamily.Error.Is matches on code+family, so two owned sentinels
	// sharing both would be indistinguishable via errors.Is. Pin uniqueness.
	type identity struct {
		family errorfamily.Family
		code   string
	}

	seen := make(map[identity]error)

	for sentinel := range auditlog.ErrorClassifications() {
		typed, ok := errors.AsType[*errorfamily.Error](sentinel)
		if !ok {
			continue
		}

		combo := identity{typed.Family(), typed.Code()}

		if prev, dup := seen[combo]; dup {
			t.Errorf("duplicate identity %v claimed by both %v and %v", combo, prev, sentinel)
		}

		seen[combo] = sentinel
	}
}

func TestClassify_ErrFileExistsIsRejectionWithWriteChain(t *testing.T) {
	t.Parallel()

	// ErrFileExists carries Rejection intrinsically (the caller asked for an
	// impossible no-clobber write). Before intrinsic classification it fell
	// through its cause chain to ErrExportWriteFailed (Infrastructure), so
	// no-clobber rejections were misreported as retryable=false but with
	// Infrastructure exit code 69 instead of Rejection exit code 1.
	if got := errorfamily.Classify(auditlog.ErrFileExists); got != errorfamily.Rejection {
		t.Errorf("Classify(ErrFileExists) = %v, want Rejection", got)
	}

	if got := errorfamily.ExitCode(auditlog.ErrFileExists); got != 1 {
		t.Errorf("ExitCode(ErrFileExists) = %d, want 1", got)
	}

	// The cause chain still contains ErrExportWriteFailed so broad matching
	// on the write-failure parent keeps working.
	if !errors.Is(auditlog.ErrFileExists, auditlog.ErrExportWriteFailed) {
		t.Error("errors.Is(ErrFileExists, ErrExportWriteFailed) = false, want true (cause chain preserved)")
	}

	wrapped := fmt.Errorf("%w: %q", auditlog.ErrFileExists, "out.json")

	if !errors.Is(wrapped, auditlog.ErrFileExists) {
		t.Error("errors.Is(wrapped, ErrFileExists) = false, want true")
	}

	if got := errorfamily.Classify(wrapped); got != errorfamily.Rejection {
		t.Errorf("Classify(wrapped ErrFileExists) = %v, want Rejection", got)
	}
}

func TestClassify_MigrationSentinelsAreRejection(t *testing.T) {
	t.Parallel()

	// Both migration sentinels are bad-caller-input errors. They were
	// previously unclassified (falling through to the Transient fail-open
	// default), misreporting caller mistakes as retryable.
	tests := []struct {
		name string
		err  error
	}{
		{name: "ErrMigrationEmptyInput", err: auditlog.ErrMigrationEmptyInput},
		{name: "ErrMigrationMissingVersion", err: auditlog.ErrMigrationMissingVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := errorfamily.Classify(tt.err); got != errorfamily.Rejection {
				t.Errorf("Classify(%s) = %v, want Rejection", tt.name, got)
			}

			if got := errorfamily.ExitCode(tt.err); got != 1 {
				t.Errorf("ExitCode(%s) = %d, want 1", tt.name, got)
			}

			if errorfamily.IsRetryable(tt.err) {
				t.Errorf("IsRetryable(%s) = true, want false (bad input is not retryable)", tt.name)
			}

			if !errors.Is(tt.err, tt.err) {
				t.Errorf("errors.Is(%s, itself) = false, want true", tt.name)
			}
		})
	}
}
