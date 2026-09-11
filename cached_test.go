package auditlog_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

func TestMarkCached_MarksStepAttemptEndEventAndCount(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	w.Add(flow.Step(testhelpers.NewCached("cache-hit")))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	step := testhelpers.FindStep(t, report, "cache-hit")
	if !step.Cached {
		t.Error("expected step.Cached=true after MarkCached")
	}

	if step.Status != auditlog.StepStatusSucceeded {
		t.Errorf("cached flag must not change status: got %q", step.Status)
	}

	if report.CachedStepCount != 1 {
		t.Errorf("expected CachedStepCount=1, got %d", report.CachedStepCount)
	}

	var cachedEnds int

	for _, evt := range report.Events {
		if evt.IsAttemptEnd() && evt.WasCached() {
			cachedEnds++
		}

		if evt.IsAttemptStart() && evt.Cached {
			t.Error("attempt_start events must never carry the cached flag")
		}
	}

	if cachedEnds != 1 {
		t.Errorf("expected exactly 1 cached attempt_end event, got %d", cachedEnds)
	}
}

func TestMarkCached_ForeignContext(t *testing.T) {
	t.Parallel()

	err := auditlog.MarkCached(context.Background())
	if !errors.Is(err, auditlog.ErrMarkCachedNoStepContext) {
		t.Fatalf("expected ErrMarkCachedNoStepContext, got %v", err)
	}
}

func TestMarkCached_DisabledAuditorInjectsNothing(t *testing.T) {
	t.Parallel()

	// A disabled auditor never injects the step context, so a step that
	// unconditionally calls MarkCached surfaces the sentinel instead of
	// failing silently. Consumers treat it as non-fatal.
	a, err := auditlog.New(auditlog.Config{WorkflowID: "disabled-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	w := &flow.Workflow{}
	step := testhelpers.NewCached("no-audit")
	w.Add(flow.Step(step))
	a.Attach(w)

	if err := w.Do(t.Context()); err == nil {
		t.Fatal("expected step to fail with ErrMarkCachedNoStepContext when auditing is disabled")
	}
}

func TestMarkCached_DoesNotLeakAcrossSteps(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	testhelpers.AddParallelSteps(w, testhelpers.NewCached("cached-one"), testhelpers.NewSucceed("fresh-two"))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	if got := testhelpers.FindStep(t, report, "cached-one"); !got.Cached {
		t.Error("cached-one should be marked cached")
	}

	if got := testhelpers.FindStep(t, report, "fresh-two"); got.Cached {
		t.Error("fresh-two must not inherit the cached flag of a parallel step")
	}

	if report.CachedStepCount != 1 {
		t.Errorf("expected CachedStepCount=1, got %d", report.CachedStepCount)
	}
}

func TestMarkCached_ConcurrentSteps(t *testing.T) {
	t.Parallel()

	const steps = 16

	a, w := testhelpers.NewAuditAndWorkflow(t)

	// No dependencies: go-workflow runs these steps concurrently, so
	// MarkCached fires from many goroutines at once.
	for range steps {
		w.Add(flow.Step(testhelpers.NewCached("parallel-cached")))
	}

	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	if report.CachedStepCount != steps {
		t.Errorf("expected CachedStepCount=%d, got %d", steps, report.CachedStepCount)
	}

	for _, step := range report.Steps {
		if !step.Cached {
			t.Errorf("step %q should be cached", step.Name)
		}
	}
}

// flakyThenCachedStep fails its first attempt (executing "work") and serves
// the second attempt from cache — modeling a cold-then-warm cache under retry.
type flakyThenCachedStep struct {
	name  string
	calls int
}

func (s *flakyThenCachedStep) Do(ctx context.Context) error {
	s.calls++

	if s.calls == 1 {
		return testhelpers.TestError("cold cache: executed and failed")
	}

	return auditlog.MarkCached(ctx)
}

func (s *flakyThenCachedStep) String() string { return s.name }

func TestMarkCached_RetrySecondAttemptCached(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := &flakyThenCachedStep{name: "warm-after-retry"}
	w.Add(flow.Step(step).Retry(testhelpers.RetryOpts(2)))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	info := testhelpers.FindStep(t, report, "warm-after-retry")
	if !info.Cached {
		t.Error("step should be cached after the second attempt hit the cache")
	}

	if info.HasError() {
		t.Error("step error must be cleared by the successful (cached) retry")
	}

	if info.AttemptCount != 2 {
		t.Errorf("expected 2 attempts, got %d", info.AttemptCount)
	}

	// The event stream must tell the per-attempt truth: attempt 1 executed,
	// attempt 2 was served from cache.
	perAttempt := make(map[int]bool)

	for _, evt := range report.EventsByStep("warm-after-retry") {
		if evt.IsAttemptEnd() {
			perAttempt[evt.Attempt] = evt.Cached
		}
	}

	if perAttempt[1] {
		t.Error("attempt 1 executed fresh and must not be marked cached")
	}

	if !perAttempt[2] {
		t.Error("attempt 2 was served from cache and must be marked cached")
	}
}

func TestCached_DefaultsFalseAndOmittedFromJSON(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "never-cached")
	report := a.Report()

	step := testhelpers.FindStep(t, report, "never-cached")
	if step.Cached {
		t.Error("plain steps must default to Cached=false")
	}

	if report.CachedStepCount != 0 {
		t.Errorf("expected CachedStepCount=0, got %d", report.CachedStepCount)
	}

	var buf strings.Builder

	if err := report.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	if strings.Contains(buf.String(), "\"cached\":true") {
		t.Error("JSON for an uncached run must not contain \"cached\":true")
	}
}

func TestCached_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	w.Add(flow.Step(testhelpers.NewCached("roundtrip-cached")))
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	if err := a.Report().WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	loaded, err := auditlog.LoadReportFromBytes([]byte(buf.String()))
	if err != nil {
		t.Fatalf("LoadReportFromBytes: %v", err)
	}

	if err := loaded.Validate(); err != nil {
		t.Fatalf("loaded report failed validation: %v", err)
	}

	step := testhelpers.FindStep(t, loaded, "roundtrip-cached")
	if !step.Cached {
		t.Error("step.Cached must survive the JSON round-trip")
	}

	if loaded.CachedStepCount != 1 {
		t.Errorf("expected loaded CachedStepCount=1, got %d", loaded.CachedStepCount)
	}
}

func TestCached_NDJSONReplayRoundTrip(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	w.Add(flow.Step(testhelpers.NewCached("replay-cached")))
	testhelpers.RunWorkflow(t, a, w)

	var buf strings.Builder

	if err := a.Report().WriteNDJSON(&buf); err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}

	events, err := auditlog.ReadEvents(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	replayed, err := auditlog.ReplayEvents(events)
	if err != nil {
		t.Fatalf("ReplayEvents: %v", err)
	}

	step := testhelpers.FindStep(t, replayed, "replay-cached")
	if !step.Cached {
		t.Error("step.Cached must survive the NDJSON replay round-trip")
	}

	if replayed.CachedStepCount != 1 {
		t.Errorf("expected replayed CachedStepCount=1, got %d", replayed.CachedStepCount)
	}
}

func TestCached_FilterOptions(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	testhelpers.AddParallelSteps(w, testhelpers.NewCached("cached-step"), testhelpers.NewSucceed("fresh-step"))
	testhelpers.RunWorkflow(t, a, w)

	report := a.Report()

	cachedOnly := report.Filtered(auditlog.WithCachedSteps())
	if len(cachedOnly.Steps) != 1 || cachedOnly.Steps[0].Name != "cached-step" {
		t.Errorf("WithCachedSteps should keep only cached-step, got %d steps", len(cachedOnly.Steps))
	}

	if cachedOnly.CachedStepCount != 1 {
		t.Errorf("filtered CachedStepCount should be recomputed to 1, got %d", cachedOnly.CachedStepCount)
	}

	uncachedOnly := report.Filtered(auditlog.WithUncachedSteps())
	if len(uncachedOnly.Steps) != 1 || uncachedOnly.Steps[0].Name != "fresh-step" {
		t.Errorf("WithUncachedSteps should keep only fresh-step, got %d steps", len(uncachedOnly.Steps))
	}

	if uncachedOnly.CachedStepCount != 0 {
		t.Errorf("uncached-only CachedStepCount should be 0, got %d", uncachedOnly.CachedStepCount)
	}

	// Step-level filters must also restrict the event stream to the kept steps.
	for _, evt := range cachedOnly.Events {
		if evt.Name == "fresh-step" {
			t.Error("cached-only filter must drop events of uncached steps")
		}
	}
}

func TestCached_ValidateDetectsCountDrift(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "cached-drift"}, Status: auditlog.StepStatusSucceeded, Cached: true},
		},
		StepCount:       1,
		SucceededCount:  1,
		CachedStepCount: 0, // lies: one cached step, zero claimed
	}

	err := report.Validate()
	if !errors.Is(err, auditlog.ErrCountMismatch) {
		t.Fatalf("expected ErrCountMismatch for cached count drift, got %v", err)
	}
}

func TestCached_MigrationRecomputesCount(t *testing.T) {
	t.Parallel()

	// A stale report claiming zero cached steps while carrying one is
	// repaired by MigrateReport's re-derivation.
	stale := `{
		"version": "0.1.0",
		"workflow_id": "stale",
		"exported_at": "2026-09-11T00:00:00Z",
		"steps": [
			{"step_name": "stale-cached", "status": "succeeded", "cached": true, "has_retry": false, "has_timeout": false}
		]
	}`

	migrated, err := auditlog.MigrateReport([]byte(stale))
	if err != nil {
		t.Fatalf("MigrateReport: %v", err)
	}

	if migrated.CachedStepCount != 1 {
		t.Errorf("migration should recompute CachedStepCount=1, got %d", migrated.CachedStepCount)
	}

	if err := migrated.Validate(); err != nil {
		t.Fatalf("migrated report failed validation: %v", err)
	}
}

func TestEvent_WasCached(t *testing.T) {
	t.Parallel()

	cached := auditlog.Event{Cached: true}
	if !cached.WasCached() {
		t.Error("WasCached should mirror the Cached field")
	}

	if (auditlog.Event{}).WasCached() {
		t.Error("zero-value event must not report WasCached")
	}
}
