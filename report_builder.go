package auditlog

import (
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
	version, workflowID, runID string,
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
	report.TotalDurationMs = 0

	pendingOrRunning := 0

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
		case StepStatusPending, StepStatusRunning:
			pendingOrRunning++
		}

		report.TotalDurationMs += step.Duration()
	}

	// WorkflowSucceeded requires every step to have reached a successful
	// terminal state. Pending/running steps mean the workflow isn't done yet;
	// any failure or cancel means it didn't succeed.
	report.WorkflowSucceeded = report.FailedCount == 0 &&
		report.CanceledCount == 0 &&
		pendingOrRunning == 0
}
