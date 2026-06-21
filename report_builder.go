package auditlog

import (
	"fmt"
	"slices"
	"strings"
	"time"

	flow "github.com/Azure/go-workflow"
)

// BuildReport assembles a machine-readable WorkflowReport from all captured
// events and step records.
func (r *Recorder) BuildReport() WorkflowReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	steps := r.buildStepsLocked()
	events := append([]Event(nil), r.events...)

	return buildReportFromCore(
		SchemaVersion,
		r.workflowID,
		r.runID,
		time.Now(),
		r.droppedEvents.Load(),
		events,
		steps,
	)
}

// buildStepsLocked assembles sorted StepInfo from the recorded data.
// Must be called with r.mu held for reading.
func (r *Recorder) buildStepsLocked() []StepInfo {
	dependents := r.buildDependentsMapLocked()

	steps := make([]StepInfo, 0, len(r.steps))

	for step, rec := range r.steps {
		deps := append([]StepRef(nil), rec.dependencies...)
		sortByName(deps)

		stepDeps := dependents[step]
		sortByName(stepDeps)

		info := stepRecordToInfo(rec)
		info.Dependencies = deps
		info.Dependents = stepDeps
		steps = append(steps, info)
	}

	// Sort by name for deterministic output.
	sortStepsByName(steps)

	return steps
}

// buildDependentsMapLocked computes reverse dependencies: for each step,
// which other steps depend on it. Returns a map of step → dependent refs.
// Must be called with r.mu held for reading.
func (r *Recorder) buildDependentsMapLocked() map[flow.Steper][]StepRef {
	nameToStep := make(map[string]flow.Steper)
	for step, rec := range r.steps {
		nameToStep[rec.Name] = step
	}

	dependents := make(map[flow.Steper][]StepRef)

	for _, rec := range r.steps {
		for _, dep := range rec.dependencies {
			if targetStep, ok := nameToStep[dep.Name]; ok {
				dependents[targetStep] = append(dependents[targetStep], StepRef{
					Name:     rec.Name,
					StepType: rec.StepType,
				})
			}
		}
	}

	return dependents
}

// stepRecordToInfo converts an internal stepRecord to a public StepInfo.
func stepRecordToInfo(rec *stepRecord) StepInfo {
	return StepInfo{
		StepRef:      rec.StepRef,
		StepID:       rec.stepID,
		Status:       rec.status,
		AttemptCount: rec.attemptCount,
		MaxAttempts:  rec.maxAttempts,
		StartedAt:    rec.startedAt,
		FinishedAt:   rec.finishedAt,
		DurationMs:   rec.durationMs,
		Error:        rec.attemptErr,
		HasRetry:     rec.hasRetry,
		HasTimeout:   rec.hasTimeout,
	}
}

// buildReportFromCore assembles a WorkflowReport from core data, deriving all
// denormalized aggregate fields. This is the single construction path.
func buildReportFromCore(
	version, workflowID string, runID RunID,
	exportedAt time.Time,
	droppedEventCount int64,
	events []Event,
	steps []StepInfo,
) WorkflowReport {
	report := WorkflowReport{
		Version:           version,
		WorkflowID:        workflowID,
		RunID:             runID,
		ExportedAt:        exportedAt,
		DroppedEventCount: droppedEventCount,
		Events:            events,
		Steps:             steps,
	}
	finalizeDenormalized(&report)

	return report
}

// finalizeDenormalized recomputes all aggregate fields from the core data.
func finalizeDenormalized(report *WorkflowReport) {
	report.EventCount = len(report.Events)
	report.StepCount = len(report.Steps)

	report.SucceededCount = 0
	report.FailedCount = 0
	report.SkippedCount = 0
	report.CanceledCount = 0
	report.PendingCount = 0
	report.RunningCount = 0
	report.TotalDurationMs = 0

	for _, step := range report.Steps {
		switch step.Status {
		case StepStatusSucceeded:
			report.SucceededCount++
		case StepStatusFailed:
			report.FailedCount++
		case StepStatusSkipped:
			report.SkippedCount++
		case StepStatusCanceled:
			report.CanceledCount++
		case StepStatusPending:
			report.PendingCount++
		case StepStatusRunning:
			report.RunningCount++
		}

		report.TotalDurationMs += step.Duration()
	}

	// WorkflowSucceeded requires every step to have reached a successful
	// terminal state. Pending/running steps mean the workflow isn't done yet;
	// any failure or cancel means it didn't succeed.
	report.WorkflowSucceeded = report.FailedCount == 0 &&
		report.CanceledCount == 0 &&
		report.PendingCount == 0 &&
		report.RunningCount == 0

	report.PeakConcurrency = computePeakConcurrency(report.Events)
	report.CriticalPathDurationMs = computeCriticalPathDuration(report.Steps)
	report.WallClockDurationMs = computeWallClockDurationMs(report.Events)
	report.FailureReason = buildFailureReason(*report)
}

// computePeakConcurrency scans the event stream and returns the maximum number
// of attempts that were in-flight simultaneously.
func computePeakConcurrency(events []Event) int {
	if len(events) == 0 {
		return 0
	}

	// Sort by timestamp, using sequence as tiebreaker for determinism.
	sorted := append([]Event(nil), events...)
	slices.SortFunc(sorted, func(a, b Event) int {
		if cmp := a.Timestamp.Compare(b.Timestamp); cmp != 0 {
			return cmp
		}

		return a.Sequence - b.Sequence
	})

	peak := 0
	inFlight := 0

	for _, evt := range sorted {
		switch evt.EventType {
		case EventTypeAttemptStart:
			inFlight++
			if inFlight > peak {
				peak = inFlight
			}
		case EventTypeAttemptEnd:
			inFlight--
			if inFlight < 0 {
				inFlight = 0
			}
		}
	}

	return peak
}

// computeCriticalPathDuration calculates the longest dependency-chain duration
// in milliseconds. This represents the minimum possible wall-clock time given
// perfect parallelization — the bottleneck path through the DAG.
func computeCriticalPathDuration(steps []StepInfo) float64 {
	if len(steps) == 0 {
		return 0
	}

	// Build name -> step lookup.
	byName := make(map[string]*StepInfo, len(steps))
	for i := range steps {
		byName[steps[i].Name] = &steps[i]
	}

	// memo stores the longest path duration ending at each step.
	memo := make(map[string]float64)

	var dfs func(name string) float64

	dfs = func(name string) float64 {
		if d, ok := memo[name]; ok {
			return d
		}

		step, ok := byName[name]
		if !ok {
			return 0
		}

		maxDepDuration := 0.0

		for _, dep := range step.Dependencies {
			d := dfs(dep.Name)
			if d > maxDepDuration {
				maxDepDuration = d
			}
		}

		memo[name] = maxDepDuration + step.Duration()

		return memo[name]
	}

	criticalPath := 0.0

	for i := range steps {
		d := dfs(steps[i].Name)
		if d > criticalPath {
			criticalPath = d
		}
	}

	return criticalPath
}

// buildFailureReason returns a human-readable summary when the workflow failed.
func buildFailureReason(report WorkflowReport) string {
	if report.WorkflowSucceeded {
		return ""
	}

	var failed []string

	var canceled []string

	for _, step := range report.Steps {
		switch step.Status {
		case StepStatusFailed:
			failed = append(failed, step.Name)
		case StepStatusCanceled:
			canceled = append(canceled, step.Name)
		default:
			// ignore non-error statuses
		}
	}

	var parts []string

	if len(failed) > 0 {
		parts = append(parts, fmt.Sprintf("%d step(s) failed: %s", len(failed), strings.Join(failed, ", ")))
	}

	if len(canceled) > 0 {
		parts = append(parts, fmt.Sprintf("%d step(s) canceled: %s", len(canceled), strings.Join(canceled, ", ")))
	}

	if len(parts) == 0 {
		if report.PendingCount > 0 || report.RunningCount > 0 {
			return fmt.Sprintf("%d step(s) still pending or running", report.PendingCount+report.RunningCount)
		}

		return "workflow did not succeed"
	}

	return strings.Join(parts, "; ")
}
