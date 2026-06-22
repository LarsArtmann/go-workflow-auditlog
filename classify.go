package auditlog

import errorfamily "github.com/larsartmann/go-error-family"

// RegisterClassifications registers all auditlog sentinel errors into the
// provided registry with their behavioral [errorfamily.Family] classification.
//
// Consumers using a custom [errorfamily.Registry] (rather than the package-level
// [errorfamily.DefaultRegistry]) must call this to receive classification
// metadata. For the common case, auditlog's [init] already registers into
// [errorfamily.DefaultRegistry], so most consumers never need to call this.
func RegisterClassifications(reg *errorfamily.Registry) {
	reg.RegisterClassifications(ErrorClassifications())
}

// ErrorClassifications returns the canonical mapping of auditlog sentinel
// errors to their behavioral [errorfamily.Family].
//
// The mapping encodes domain knowledge that only auditlog owns: whether a
// given error is the caller's fault ([errorfamily.Rejection] — bad input),
// a data-integrity violation ([errorfamily.Corruption] — structurally invalid
// report), a transient failure ([errorfamily.Transient] — retryable), or a
// system-level failure ([errorfamily.Infrastructure] — not retryable).
func ErrorClassifications() map[error]errorfamily.Family {
	return map[error]errorfamily.Family{
		// Corruption — internal data integrity violations. The report is
		// structurally invalid; no caller action can fix it.
		ErrEventCountMismatch: errorfamily.Corruption,
		ErrStepCountMismatch:  errorfamily.Corruption,
		ErrStatusDrift:        errorfamily.Corruption,
		ErrCountMismatch:      errorfamily.Corruption,

		// Rejection — bad caller input. The caller sent empty data, oversized
		// input, invalid config, or asked for an impossible operation.
		ErrEmpty:             errorfamily.Rejection,
		ErrNoEvents:          errorfamily.Rejection,
		ErrOversizedLine:     errorfamily.Rejection,
		ErrWorkflowIDPathSep: errorfamily.Rejection,
		ErrReplayNoEvents:    errorfamily.Rejection,

		// Transient — temporary failure, worth retrying.
		ErrReportLoadFailed: errorfamily.Transient,

		// Infrastructure — system-level failure, not retryable.
		ErrRenderFailed:      errorfamily.Infrastructure,
		ErrExportWriteFailed: errorfamily.Infrastructure,

		// Private sentinels — classified for completeness so that wrapped
		// errors carrying these through fmt.Errorf("%w") are classified.
		errUnknownEventType: errorfamily.Rejection,
		errUnknownPhase:     errorfamily.Rejection,
	}
}

// init registers all sentinel error classifications into the
// [errorfamily.DefaultRegistry] so that consumers who import auditlog
// automatically get [errorfamily.Classify], [errorfamily.IsRetryable], and
// [errorfamily.ExitCode] on auditlog errors without any additional setup.
// This follows the standard Go driver-registration pattern (cf. database/sql,
// image codec registration). Consumers who prefer a separate registry should
// call [RegisterClassifications] with their own [errorfamily.Registry].
//
//nolint:gochecknoinits // Standard Go self-registration pattern.
func init() {
	RegisterClassifications(errorfamily.DefaultRegistry)
}
