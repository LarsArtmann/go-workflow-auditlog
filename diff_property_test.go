package auditlog_test

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// These tests exercise algebraic properties of WorkflowReport.Diff over many
// randomly-generated report pairs. A fixed seed makes failures reproducible.

var diffStepStatuses = []auditlog.StepStatus{
	auditlog.StepStatusPending,
	auditlog.StepStatusRunning,
	auditlog.StepStatusSucceeded,
	auditlog.StepStatusFailed,
	auditlog.StepStatusCanceled,
	auditlog.StepStatusSkipped,
}

var diffStepNames = []string{"fetch", "validate", "transform", "save", "notify", "cleanup", "retry", "deploy"}

// randWorkflowReport builds a pseudo-random WorkflowReport from a deterministic
// RNG. Only the fields Diff inspects are populated: Steps, WallClockDurationMs,
// CriticalPathDurationMs, PeakConcurrency, and CriticalPathSteps.
func randWorkflowReport(rng *rand.Rand) auditlog.WorkflowReport {
	n := rng.IntN(len(diffStepNames) + 1)
	namePool := slices.Clone(diffStepNames)

	for i := len(namePool) - 1; i > 0; i-- {
		j := rng.IntN(i + 1)
		namePool[i], namePool[j] = namePool[j], namePool[i]
	}

	steps := make([]auditlog.StepInfo, 0, n)
	for i := range n {
		steps = append(steps, auditlog.StepInfo{
			StepRef: auditlog.StepRef{Name: namePool[i]},
			Status:  diffStepStatuses[rng.IntN(len(diffStepStatuses))],
		})
	}

	// Randomly select a subset of step names as the critical path.
	critN := rng.IntN(n + 1)
	criticalPath := make([]string, 0, critN)
	for i := range critN {
		criticalPath = append(criticalPath, namePool[i])
	}

	return auditlog.WorkflowReport{
		Steps:                  steps,
		WallClockDurationMs:    float64(rng.IntN(50000)),
		CriticalPathDurationMs: float64(rng.IntN(50000)),
		PeakConcurrency:        rng.IntN(16),
		CriticalPathSteps:      criticalPath,
	}
}

func TestDiff_Identity(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(1, 1))

	for range 200 {
		a := randWorkflowReport(rng)
		if d := a.Diff(a); d.HasChanges() {
			t.Fatalf("Diff(a,a) must have no changes, got %+v", d)
		}
	}
}

func TestDiff_AddedRemovedDuality(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(2, 2))

	for range 200 {
		a, b := randWorkflowReport(rng), randWorkflowReport(rng)
		forward, reverse := a.Diff(b), b.Diff(a)

		if !stepNamesEqual(forward.AddedSteps, reverse.RemovedSteps) {
			t.Errorf("Added(a→b) != Removed(b→a)\n  forward added: %v\n  reverse removed: %v",
				stepDiffNames(forward.AddedSteps), stepDiffNames(reverse.RemovedSteps))
		}

		if !stepNamesEqual(forward.RemovedSteps, reverse.AddedSteps) {
			t.Errorf("Removed(a→b) != Added(b→a)\n  forward removed: %v\n  reverse added: %v",
				stepDiffNames(forward.RemovedSteps), stepDiffNames(reverse.AddedSteps))
		}
	}
}

func TestDiff_DurationAntiSymmetry(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(3, 3))

	for range 200 {
		a, b := randWorkflowReport(rng), randWorkflowReport(rng)
		forward, reverse := a.Diff(b), b.Diff(a)

		if forward.DurationDelta != -reverse.DurationDelta {
			t.Errorf("Δ(a→b)=%.2f should equal -Δ(b→a)=%.2f",
				forward.DurationDelta, -reverse.DurationDelta)
		}
	}
}

func TestDiff_StatusChangedSymmetry(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(4, 4))

	for range 200 {
		a, b := randWorkflowReport(rng), randWorkflowReport(rng)
		forward, reverse := a.Diff(b), b.Diff(a)

		fwdBy := indexStepDiffs(forward.StatusChanged)
		revBy := indexStepDiffs(reverse.StatusChanged)

		if len(fwdBy) != len(revBy) {
			t.Fatalf("changed count mismatch: forward=%d reverse=%d", len(fwdBy), len(revBy))
		}

		for key, fd := range fwdBy {
			rd, ok := revBy[key]
			if !ok {
				t.Fatalf("step %q changed forward but not reverse", key)
			}

			// Status in forward = new (from b), OldStatus = from a.
			// Status in reverse = new (from a), OldStatus = from b.
			// So forward.Status should equal reverse.OldStatus and vice versa.
			if fd.Status != rd.OldStatus {
				t.Errorf("step %q: forward Status=%s should equal reverse OldStatus=%s",
					key, fd.Status, rd.OldStatus)
			}

			if fd.OldStatus != rd.Status {
				t.Errorf("step %q: forward OldStatus=%s should equal reverse Status=%s",
					key, fd.OldStatus, rd.Status)
			}
		}
	}
}

func TestDiff_OutputSorted(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(5, 5))

	for range 200 {
		a, b := randWorkflowReport(rng), randWorkflowReport(rng)
		d := a.Diff(b)

		if !slices.IsSortedFunc(d.AddedSteps, cmpStepDiff) {
			t.Error("AddedSteps not sorted by name")
		}

		if !slices.IsSortedFunc(d.RemovedSteps, cmpStepDiff) {
			t.Error("RemovedSteps not sorted by name")
		}

		if !slices.IsSortedFunc(d.StatusChanged, cmpStepDiff) {
			t.Error("StatusChanged not sorted by name")
		}

		if !slices.IsSortedFunc(d.CriticalPathStepsAdded, cmp.Compare) {
			t.Error("CriticalPathStepsAdded not sorted by name")
		}

		if !slices.IsSortedFunc(d.CriticalPathStepsRemoved, cmp.Compare) {
			t.Error("CriticalPathStepsRemoved not sorted by name")
		}
	}
}

func TestDiff_CriticalPathAntiSymmetry(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(6, 6))

	for range 200 {
		a, b := randWorkflowReport(rng), randWorkflowReport(rng)
		forward, reverse := a.Diff(b), b.Diff(a)

		if forward.CriticalPathDeltaMs != -reverse.CriticalPathDeltaMs {
			t.Errorf("CriticalPathDelta(a→b)=%.2f should equal -Δ(b→a)=%.2f",
				forward.CriticalPathDeltaMs, -reverse.CriticalPathDeltaMs)
		}
	}
}

func TestDiff_PeakConcurrencyAntiSymmetry(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(7, 7))

	for range 200 {
		a, b := randWorkflowReport(rng), randWorkflowReport(rng)
		forward, reverse := a.Diff(b), b.Diff(a)

		if forward.PeakConcurrencyDelta != -reverse.PeakConcurrencyDelta {
			t.Errorf("PeakConcurrencyDelta(a→b)=%d should equal -Δ(b→a)=%d",
				forward.PeakConcurrencyDelta, -reverse.PeakConcurrencyDelta)
		}
	}
}

func TestDiff_CriticalPathStepsDuality(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(8, 8))

	for range 200 {
		a, b := randWorkflowReport(rng), randWorkflowReport(rng)
		forward, reverse := a.Diff(b), b.Diff(a)

		if !stringSetEqual(forward.CriticalPathStepsAdded, reverse.CriticalPathStepsRemoved) {
			t.Errorf("CriticalPathStepsAdded(a→b) != Removed(b→a)\n  added: %v\n  removed: %v",
				forward.CriticalPathStepsAdded, reverse.CriticalPathStepsRemoved)
		}

		if !stringSetEqual(forward.CriticalPathStepsRemoved, reverse.CriticalPathStepsAdded) {
			t.Errorf("CriticalPathStepsRemoved(a→b) != Added(b→a)\n  removed: %v\n  added: %v",
				forward.CriticalPathStepsRemoved, reverse.CriticalPathStepsAdded)
		}
	}
}

// --- helpers ---

func stepNamesEqual(a, b []auditlog.StepDiff) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
	}

	return true
}

func stepDiffNames(diffs []auditlog.StepDiff) []string {
	names := make([]string, 0, len(diffs))
	for _, d := range diffs {
		names = append(names, d.Name)
	}

	return names
}

func indexStepDiffs(diffs []auditlog.StepDiff) map[string]auditlog.StepDiff {
	out := make(map[string]auditlog.StepDiff, len(diffs))
	for _, d := range diffs {
		out[d.Name] = d
	}

	return out
}

func cmpStepDiff(a, b auditlog.StepDiff) int {
	if a.Name < b.Name {
		return -1
	}

	if a.Name > b.Name {
		return 1
	}

	return 0
}

func stringSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	set := make(map[string]struct{}, len(a))
	for _, s := range a {
		set[s] = struct{}{}
	}

	for _, s := range b {
		if _, ok := set[s]; !ok {
			return false
		}
	}

	return true
}
