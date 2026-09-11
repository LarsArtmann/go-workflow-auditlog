package auditlog

import errorfamily "github.com/larsartmann/go-error-family"

// RegisterClassifications registers all auditlog sentinel errors into the
// provided registry with their behavioral [errorfamily.Family] classification.
//
// Errors owned by auditlog carry their family intrinsically (they implement
// the [errorfamily.Classified] interface), so registration is only required
// for the re-exported go-ndjson sentinels ([ErrEmpty], [ErrNoEvents],
// [ErrOversizedLine]) — third-party errors that auditlog does not own. For
// the common case, auditlog's [init] already registers into
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
//
// Owned sentinels carry the same family intrinsically — the registry is a
// belt-and-braces back-compat layer, consulted only for errors that do not
// implement [errorfamily.Classified] themselves (e.g. the re-exported
// go-ndjson sentinels). A sync test pins the map and the intrinsic families
// together.
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
		// ErrEmpty/ErrNoEvents/ErrOversizedLine are go-ndjson's errors (not
		// owned here) — the registry is their only classification channel.
		ErrEmpty:                 errorfamily.Rejection,
		ErrNoEvents:              errorfamily.Rejection,
		ErrOversizedLine:         errorfamily.Rejection,
		ErrWorkflowIDPathSep:     errorfamily.Rejection,
		ErrReplayNoEvents:        errorfamily.Rejection,
		ErrMigrationEmptyInput:   errorfamily.Rejection,
		ErrMigrationMissingVersion: errorfamily.Rejection,
		ErrFileExists:            errorfamily.Rejection,

		// Transient — temporary failure, worth retrying.
		ErrReportLoadFailed: errorfamily.Transient,

		// Infrastructure — system-level failure, not retryable.
		ErrRenderFailed:      errorfamily.Infrastructure,
		ErrExportWriteFailed: errorfamily.Infrastructure,

		// Private sentinels — classified for completeness so that wrapped
		// errors carrying these through fmt.Errorf("%w") are classified.
		errUnknownEventType:  errorfamily.Rejection,
		errUnknownPhase:      errorfamily.Rejection,
		errNilStreamCallback: errorfamily.Rejection,
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
