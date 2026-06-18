package auditlog

import (
	"fmt"
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
type StepDiff struct {
	Name     string     `json:"name"`
	Status   StepStatus `json:"status"`
	Duration float64    `json:"duration_ms,omitempty"`
}

// HasChanges returns true if the diff found any differences.
func (d DiffResult) HasChanges() bool {
	return len(d.AddedSteps) > 0 || len(d.RemovedSteps) > 0 ||
		len(d.StatusChanged) > 0 || d.DurationDelta != 0
}

// Diff compares this report against another and returns the differences.
// Useful for detecting regressions between workflow runs.
func (r WorkflowReport) Diff(other WorkflowReport) DiffResult {
	result := DiffResult{
		DurationDelta: other.TotalDurationMs - r.TotalDurationMs,
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

	// Find added and changed steps.
	for name, theirStep := range theirs {
		if ourStep, ok := ours[name]; ok {
			if ourStep.Status != theirStep.Status {
				result.StatusChanged = append(result.StatusChanged, diffStep(name, theirStep))
			}
		} else {
			result.AddedSteps = append(result.AddedSteps, diffStep(name, theirStep))
		}
	}

	// Find removed steps.
	for name, ourStep := range ours {
		if _, ok := theirs[name]; !ok {
			result.RemovedSteps = append(result.RemovedSteps, diffStep(name, ourStep))
		}
	}

	return result
}

// diffStep builds a StepDiff entry from a step name and StepInfo.
func diffStep(name string, step StepInfo) StepDiff {
	return StepDiff{
		Name:     name,
		Status:   step.Status,
		Duration: step.Duration(),
	}
}

// Duration returns the total wall-clock duration spanned by all events,
// from the earliest to the latest timestamp. This is different from
// TotalDurationMs (which sums individual step durations and may overcount
// when steps run in parallel).
func (r WorkflowReport) Duration() time.Duration {
	if len(r.Events) == 0 {
		return 0
	}

	earliest := r.Events[0].Timestamp
	latest := r.Events[0].Timestamp

	for _, evt := range r.Events {
		if evt.Timestamp.Before(earliest) {
			earliest = evt.Timestamp
		}

		if evt.Timestamp.After(latest) {
			latest = evt.Timestamp
		}
	}

	return latest.Sub(earliest)
}

// Summary returns a human-readable one-line summary of the report.
func (r WorkflowReport) Summary() string {
	return fmt.Sprintf("%s: %d steps (%d ok, %d failed, %d skipped) in %.1fms",
		r.WorkflowID, r.StepCount, r.SucceededCount, r.FailedCount,
		r.SkippedCount, r.TotalDurationMs)
}
