package auditlog

import (
	"context"
	"slices"
	"time"

	flow "github.com/Azure/go-workflow"
)

// Attach injects audit BeforeStep/AfterStep callbacks into every step in the
// workflow. Call this BEFORE w.Do(ctx).
//
// The callbacks are merged into each step's existing config via State.MergeConfig,
// so user-defined callbacks (Input, Output, BeforeStep, AfterStep) are preserved.
// Audit callbacks are appended last so they observe the final error.
//
// When the Auditor is disabled, Attach is a no-op.
func (a *Auditor) Attach(w *flow.Workflow) *flow.Workflow {
	if !a.config.Enabled || w == nil {
		return w
	}

	for _, step := range w.Steps() {
		state := w.StateOf(step)
		if state == nil {
			continue
		}

		state.MergeConfig(&flow.StepConfig{
			Before: []flow.BeforeStep{a.beforeFn},
			After:  []flow.AfterStep{a.afterFn},
		})
	}

	return w
}

// Snapshot reads the workflow's final state after Do() to capture the full DAG
// structure, final statuses, and any steps that were skipped or canceled
// (which bypass Before/After callbacks entirely).
//
// Call this AFTER w.Do(ctx) returns.
//
// When the Auditor is disabled, Snapshot is a no-op.
func (a *Auditor) Snapshot(w *flow.Workflow) {
	if !a.config.Enabled || w == nil {
		return
	}

	a.recorder.snapshotWorkflow(w)
}

// makeCallbacks creates the BeforeStep and AfterStep closures that feed the recorder.
func (r *Recorder) makeCallbacks() (flow.BeforeStep, flow.AfterStep) {
	before := func(ctx context.Context, step flow.Steper) (context.Context, error) {
		r.recordBeforeStep(step)

		return ctx, nil
	}
	after := func(_ context.Context, step flow.Steper, err error) error {
		r.recordAfterStep(step, err)

		return err
	}

	return before, after
}

// snapshotWorkflow reads step statuses, dependencies, and retry/timeout config
// from the workflow's post-execution state.
func (r *Recorder) snapshotWorkflow(w *flow.Workflow) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, step := range w.Steps() {
		r.snapshotStepLocked(w, step)
	}
}

// snapshotStepLocked captures a single step's final state from the workflow.
// Caller must hold r.mu.
func (r *Recorder) snapshotStepLocked(w *flow.Workflow, step flow.Steper) {
	name := flow.String(step)

	state := w.StateOf(step)
	if state == nil {
		return
	}

	status := fromFlowStatus(string(state.GetStatus()))
	err := state.GetError()

	now := time.Now()
	rec := r.getOrCreateStepLocked(step, name, now)
	rec.status = status

	if err != nil {
		errStr := err.Error()
		rec.attemptErr = &errStr
	}

	// Capture retry and timeout configuration.
	if opt := state.Option(); opt != nil {
		if opt.RetryOption != nil {
			rec.hasRetry = true
			rec.maxAttempts = int(opt.RetryOption.Attempts)
		}

		if opt.Timeout != nil {
			rec.hasTimeout = true
		}
	}

	// Capture dependencies (upstream steps).
	upstreams := w.UpstreamOf(step)
	deps := make([]string, 0, len(upstreams))

	for up := range upstreams {
		deps = append(deps, flow.String(up))
	}

	slices.Sort(deps)
	rec.dependencies = deps
}
