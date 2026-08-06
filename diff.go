package auditlog

import (
	"slices"
)

// DiffResult describes the differences between two workflow reports.
type DiffResult struct {
	AddedSteps               []StepDiff `json:"added_steps,omitempty"`
	RemovedSteps             []StepDiff `json:"removed_steps,omitempty"`
	StatusChanged            []StepDiff `json:"status_changed,omitempty"`
	DurationDelta            float64    `json:"duration_delta_ms"`
	CriticalPathDeltaMs      float64    `json:"critical_path_delta_ms"`
	PeakConcurrencyDelta     int        `json:"peak_concurrency_delta"`
	CriticalPathStepsAdded   []string   `json:"critical_path_steps_added,omitempty"`
	CriticalPathStepsRemoved []string   `json:"critical_path_steps_removed,omitempty"`
}

// StepDiff captures a single step's state in a diff context.
// For status changes, Status holds the new value and OldStatus the previous one
// (OldStatus is empty for added steps).
type StepDiff struct {
	Name      string     `json:"name"`
	Status    StepStatus `json:"status"`
	OldStatus StepStatus `json:"old_status,omitempty"`
	Duration  float64    `json:"duration_ms,omitempty"`
}

// HasChanges returns true if the diff found any differences.
func (d DiffResult) HasChanges() bool {
	return len(d.AddedSteps) > 0 || len(d.RemovedSteps) > 0 ||
		len(d.StatusChanged) > 0 || d.DurationDelta != 0 ||
		d.CriticalPathDeltaMs != 0 || d.PeakConcurrencyDelta != 0 ||
		len(d.CriticalPathStepsAdded) > 0 || len(d.CriticalPathStepsRemoved) > 0
}

// IsEmpty returns true when no differences were found.
// This is the logical inverse of HasChanges, provided for parity with the
// samber-do-auditlog twin API.
func (d DiffResult) IsEmpty() bool {
	return !d.HasChanges()
}

// Diff compares this report against another and returns the differences.
// Useful for detecting regressions between workflow runs.
//
// Output slices are sorted by step name for deterministic results across runs.
//
// Beyond step-level diffs, Diff also surfaces aggregate deltas:
//   - CriticalPathDeltaMs: difference in critical-path duration (longest
//     dependency chain). Positive = "other" run was slower.
//   - PeakConcurrencyDelta: difference in peak concurrent step count.
//     Positive = "other" run reached higher parallelism.
//   - CriticalPathStepsAdded/Removed: step names that appear on "other"'s
//     critical path but not on this one's (and vice versa).
func (r WorkflowReport) Diff(other WorkflowReport) DiffResult {
	result := DiffResult{
		DurationDelta:        other.WallClockDurationMs - r.WallClockDurationMs,
		CriticalPathDeltaMs:  other.CriticalPathDurationMs - r.CriticalPathDurationMs,
		PeakConcurrencyDelta: other.PeakConcurrency - r.PeakConcurrency,
	}

	result.AddedSteps, result.RemovedSteps, result.StatusChanged = diffSteps(
		r.Steps, other.Steps,
	)

	result.CriticalPathStepsAdded, result.CriticalPathStepsRemoved = diffCriticalPathMembership(
		r.CriticalPathSteps, other.CriticalPathSteps,
	)

	return result
}

// diffSteps partitions steps into added (in theirs but not ours),
// removed (in ours but not theirs), and status-changed buckets.
// All output slices are sorted by step name.
func diffSteps(ours, theirs []StepInfo) ([]StepDiff, []StepDiff, []StepDiff) {
	ourMap := make(map[string]StepInfo, len(ours))
	for _, s := range ours {
		ourMap[s.Name] = s
	}

	theirMap := make(map[string]StepInfo, len(theirs))
	for _, s := range theirs {
		theirMap[s.Name] = s
	}

	var addedNames, changedNames, removedNames []string

	for name, theirStep := range theirMap {
		if ourStep, ok := ourMap[name]; ok {
			if ourStep.Status != theirStep.Status {
				changedNames = append(changedNames, name)
			}
		} else {
			addedNames = append(addedNames, name)
		}
	}

	for name := range ourMap {
		if _, ok := theirMap[name]; !ok {
			removedNames = append(removedNames, name)
		}
	}

	slices.Sort(addedNames)
	slices.Sort(changedNames)
	slices.Sort(removedNames)

	added := make([]StepDiff, 0, len(addedNames))
	changed := make([]StepDiff, 0, len(changedNames))
	removed := make([]StepDiff, 0, len(removedNames))

	for _, name := range addedNames {
		added = append(added, diffStep(name, theirMap[name], StepStatus("")))
	}

	for _, name := range changedNames {
		changed = append(changed, diffStep(name, theirMap[name], ourMap[name].Status))
	}

	for _, name := range removedNames {
		removed = append(removed, diffStep(name, ourMap[name], StepStatus("")))
	}

	return added, removed, changed
}

// diffCriticalPathMembership returns names that are on the second report's
// critical path but not the first's (added), and vice versa (removed).
// Output slices are sorted by name.
func diffCriticalPathMembership(ours, theirs []string) ([]string, []string) {
	ourSet := make(map[string]struct{}, len(ours))
	for _, n := range ours {
		ourSet[n] = struct{}{}
	}

	theirSet := make(map[string]struct{}, len(theirs))
	for _, n := range theirs {
		theirSet[n] = struct{}{}
	}

	added := make([]string, 0, len(theirs))
	removed := make([]string, 0, len(ours))

	for name := range theirSet {
		if _, ok := ourSet[name]; !ok {
			added = append(added, name)
		}
	}

	for name := range ourSet {
		if _, ok := theirSet[name]; !ok {
			removed = append(removed, name)
		}
	}

	slices.Sort(added)
	slices.Sort(removed)

	return added, removed
}

// diffStep builds a StepDiff entry from a step name and StepInfo.
// oldStatus is the previous status (empty for added/removed entries).
func diffStep(name string, step StepInfo, oldStatus StepStatus) StepDiff {
	return StepDiff{
		Name:      name,
		Status:    step.Status,
		OldStatus: oldStatus,
		Duration:  step.Duration(),
	}
}
