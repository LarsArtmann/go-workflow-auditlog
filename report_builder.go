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
// Uses stepCore.toStepInfo() for the common fields, then adds live-only data.
func stepRecordToInfo(rec *stepRecord) StepInfo {
	info := rec.toStepInfo()
	info.StepID = rec.stepID
	info.MaxAttempts = rec.maxAttempts
	info.HasRetry = rec.hasRetry
	info.HasTimeout = rec.hasTimeout

	return info
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

// sortEventsByTime returns a copy of events sorted by timestamp ascending,
// using sequence as a deterministic tiebreaker. The copy avoids mutating the
// caller's slice.
func sortEventsByTime(events []Event) []Event {
	sorted := append([]Event(nil), events...)
	slices.SortFunc(sorted, func(a, b Event) int {
		if cmp := a.Timestamp.Compare(b.Timestamp); cmp != 0 {
			return cmp
		}

		return a.Sequence - b.Sequence
	})

	return sorted
}

// computePeakConcurrency scans the event stream and returns the maximum number
// of attempts that were in-flight simultaneously.
func computePeakConcurrency(events []Event) int {
	if len(events) == 0 {
		return 0
	}

	peak := 0
	inFlight := 0

	for _, evt := range sortEventsByTime(events) {
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

// computePeakConcurrencySteps returns the unique step names that were in-flight
// at the moment of peak concurrency. Uses reference-counted tracking so that
// overlapping retries of the same step are handled correctly.
func computePeakConcurrencySteps(events []Event) []string {
	if len(events) == 0 {
		return nil
	}

	peakAttempts := 0
	attemptsInFlight := 0
	stepRefCounts := make(map[string]int)

	var peakSteps []string

	for _, evt := range sortEventsByTime(events) {
		switch evt.EventType {
		case EventTypeAttemptStart:
			attemptsInFlight++
			stepRefCounts[evt.Name]++

			if attemptsInFlight > peakAttempts {
				peakAttempts = attemptsInFlight
				peakSteps = peakSteps[:0]

				for name, count := range stepRefCounts {
					if count > 0 {
						peakSteps = append(peakSteps, name)
					}
				}
			}

		case EventTypeAttemptEnd:
			attemptsInFlight--
			if attemptsInFlight < 0 {
				attemptsInFlight = 0
			}

			stepRefCounts[evt.Name]--
		}
	}

	slices.Sort(peakSteps)

	return peakSteps
}

// computeCriticalPathDuration calculates the longest dependency-chain duration
// in milliseconds. This represents the minimum possible wall-clock time given
// perfect parallelization — the bottleneck path through the DAG.
func computeCriticalPathDuration(steps []StepInfo) float64 {
	d, _ := computeCriticalPath(steps)

	return d
}

// criticalPathResult holds the longest path ending at a step: its total
// duration and the ordered step names from root to that step.
type criticalPathResult struct {
	duration float64
	path     []string
}

// computeCriticalPath returns both the duration and the ordered step names
// (root-to-leaf) of the longest dependency chain.
func computeCriticalPath(steps []StepInfo) (float64, []string) {
	if len(steps) == 0 {
		return 0, nil
	}

	byName := make(map[string]*StepInfo, len(steps))
	for i := range steps {
		byName[steps[i].Name] = &steps[i]
	}

	memo := make(map[string]criticalPathResult)
	dfs := buildCriticalPathDFS(byName, memo)

	var best criticalPathResult

	for i := range steps {
		r := dfs(steps[i].Name)
		if r.duration > best.duration {
			best = r
		}
	}

	return best.duration, best.path
}

// buildCriticalPathDFS returns a memoized DFS that computes the longest
// dependency-chain path ending at each step.
func buildCriticalPathDFS(
	byName map[string]*StepInfo,
	memo map[string]criticalPathResult,
) func(string) criticalPathResult {
	var dfs func(string) criticalPathResult

	dfs = func(name string) criticalPathResult {
		if r, ok := memo[name]; ok {
			return r
		}

		step, ok := byName[name]
		if !ok {
			return criticalPathResult{}
		}

		bestDep := criticalPathResult{}

		for _, dep := range step.Dependencies {
			d := dfs(dep.Name)
			if d.duration > bestDep.duration {
				bestDep = d
			}
		}

		path := make([]string, 0, len(bestDep.path)+1)
		path = append(path, bestDep.path...)
		path = append(path, name)

		result := criticalPathResult{
			duration: bestDep.duration + step.Duration(),
			path:     path,
		}

		memo[name] = result

		return result
	}

	return dfs
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
