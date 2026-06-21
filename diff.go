package auditlog

import (
	"fmt"
	"slices"
	"time"
)

// DiffResult describes the differences between two workflow reports.
type DiffResult struct {
	AddedSteps    []StepDiff `json:"added_steps,omitempty"`
	RemovedSteps  []StepDiff `json:"removed_steps,omitempty"`
	StatusChanged []StepDiff `json:"status_changed,omitempty"`
	DurationDelta float64    `json:"duration_delta_ms"`
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
		len(d.StatusChanged) > 0 || d.DurationDelta != 0
}

// Diff compares this report against another and returns the differences.
// Useful for detecting regressions between workflow runs.
//
// Output slices are sorted by step name for deterministic results across runs.
func (r WorkflowReport) Diff(other WorkflowReport) DiffResult {
	result := DiffResult{
		DurationDelta: other.WallClockDurationMs - r.WallClockDurationMs,
	}

	// Build step lookup maps by name.
	ours := make(map[string]StepInfo, len(r.Steps))
	for _, s := range r.Steps {
		ours[s.Name] = s
	}

	theirs := make(map[string]StepInfo, len(other.Steps))
	for _, s := range other.Steps {
		theirs[s.Name] = s
	}

	// Collect names for deterministic ordering.
	added := make([]string, 0)
	changed := make([]string, 0)

	for name, theirStep := range theirs {
		if ourStep, ok := ours[name]; ok {
			if ourStep.Status != theirStep.Status {
				changed = append(changed, name)
			}
		} else {
			added = append(added, name)
		}
	}

	removed := make([]string, 0)

	for name := range ours {
		if _, ok := theirs[name]; !ok {
			removed = append(removed, name)
		}
	}

	slices.Sort(added)
	slices.Sort(changed)
	slices.Sort(removed)

	for _, name := range added {
		result.AddedSteps = append(result.AddedSteps, diffStep(name, theirs[name], StepStatus("")))
	}

	for _, name := range changed {
		result.StatusChanged = append(result.StatusChanged, diffStep(name, theirs[name], ours[name].Status))
	}

	for _, name := range removed {
		result.RemovedSteps = append(result.RemovedSteps, diffStep(name, ours[name], StepStatus("")))
	}

	return result
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

// Duration returns the total wall-clock duration spanned by all events,
// from the earliest to the latest timestamp. This is different from
// TotalDurationMs (which sums individual step durations and may overcount
// when steps run in parallel). The same value is available as the
// WallClockDurationMs JSON field.
func (r WorkflowReport) Duration() time.Duration {
	return time.Duration(computeWallClockDurationMs(r.Events) * float64(time.Millisecond))
}

// Summary returns a human-readable one-line summary of the report.
// Uses wall-clock duration (actual elapsed time) rather than the summed
// per-step duration, and includes the failure reason when the workflow
// did not succeed.
func (r WorkflowReport) Summary() string {
	base := fmt.Sprintf("%s: %d steps (%d ok, %d failed, %d skipped) in %.1fms",
		r.WorkflowID, r.StepCount, r.SucceededCount, r.FailedCount,
		r.SkippedCount, r.WallClockDurationMs)

	if r.WorkflowSucceeded {
		return base
	}

	if r.FailureReason != "" {
		return base + " — " + r.FailureReason
	}

	return base + " — failed"
}
