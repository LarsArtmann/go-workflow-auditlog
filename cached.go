package auditlog

import (
	"context"

	flow "github.com/Azure/go-workflow"
	errorfamily "github.com/larsartmann/go-error-family"
)

// ErrMarkCachedNoStepContext is returned by [MarkCached] when the context was
// not injected by an attached Auditor — i.e. MarkCached was called outside a
// step body, or audit logging is disabled (Attach is then a no-op, so no step
// context is ever installed). Rejection: bad caller input, code
// "auditlog.mark_cached_no_step_context".
var ErrMarkCachedNoStepContext = errorfamily.NewRejection(
	"auditlog.mark_cached_no_step_context",
	"MarkCached requires a context injected by Auditor.Attach",
)

// stepContext carries the recorder and step identity through the context that
// BeforeStep hands to step.Do, so [MarkCached] can attribute a cache hit to
// the currently executing step without name-based lookup (names may collide;
// the step pointer cannot).
type stepContext struct {
	recorder *Recorder
	step     flow.Steper
}

// stepContextKey is the unexported context key for the injected stepContext.
type stepContextKey struct{}

// withStepContext returns a context carrying the executing step's identity.
// The wrapper delegates everything except the auditlog key — deadlines,
// cancellation, and values of the parent context are preserved, so
// step-level timeouts keep working.
func withStepContext(ctx context.Context, rec *Recorder, step flow.Steper) context.Context {
	return context.WithValue(ctx, stepContextKey{}, stepContext{recorder: rec, step: step})
}

// MarkCached marks the currently executing step as having been served from a
// result cache instead of executing its work. Call it from inside a step body
// with the context received from Do:
//
//	func (s *DetectStep) Do(ctx context.Context) error {
//		if cached, ok := resultCache.Get(s.Key()); ok {
//			s.findings = cached
//			return auditlog.MarkCached(ctx) // honest audit: this result was reused, not computed
//		}
//		// ... execute the real work ...
//	}
//
// The mark surfaces on the step's next attempt_end event ([Event.Cached]) and
// is denormalized onto the step ([StepInfo.Cached]) and the report
// ([WorkflowReport.CachedStepCount]). It does NOT change the step's status:
// a cached step still succeeds or fails on its own merits — Cached records
// where the result came from, not whether it was good.
//
// The context is injected by [Auditor.Attach]. Without it (auditing disabled,
// or a context from outside the workflow) MarkCached returns
// [ErrMarkCachedNoStepContext]. Callers that support running with audit
// logging disabled should treat this error as non-fatal.
//
// MarkCached is safe for concurrent use from parallel step goroutines.
func MarkCached(ctx context.Context) error {
	sc, ok := ctx.Value(stepContextKey{}).(stepContext)
	if !ok {
		return ErrMarkCachedNoStepContext
	}

	sc.recorder.markCached(sc.step)

	return nil
}

// markCached flags the step's record so the next attempt_end event and the
// final StepInfo report the cache hit. Unknown steps are ignored: the record
// is always created by BeforeStep before Do runs, so a missing record means
// the context belongs to a step this recorder never saw.
func (r *Recorder) markCached(step flow.Steper) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rec, ok := r.steps[step]; ok {
		rec.cached = true
	}
}
