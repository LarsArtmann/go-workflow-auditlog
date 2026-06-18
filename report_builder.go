package auditlog

import (
	"slices"
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
		deps := append([]string(nil), rec.dependencies...)
		slices.Sort(deps)

		stepDeps := dependents[step]
		slices.Sort(stepDeps)

		info := stepRecordToInfo(rec)
		info.Dependencies = deps
		info.Dependents = stepDeps
		steps = append(steps, info)
	}

	// Sort by name for deterministic output.
	slices.SortFunc(steps, func(a, b StepInfo) int {
		if a.Name != b.Name {
			return cmpStrings(a.Name, b.Name)
		}

		return 0
	})

	return steps
}

// buildDependentsMapLocked computes reverse dependencies: for each step,
// which other steps depend on it. Returns a map of step → dependent names.
// Must be called with r.mu held for reading.
func (r *Recorder) buildDependentsMapLocked() map[flow.Steper][]string {
	nameToStep := make(map[string]flow.Steper)
	for step, rec := range r.steps {
		nameToStep[rec.name] = step
	}

	dependents := make(map[flow.Steper][]string)

	for _, rec := range r.steps {
		for _, depName := range rec.dependencies {
			if targetStep, ok := nameToStep[depName]; ok {
				dependents[targetStep] = append(dependents[targetStep], rec.name)
			}
		}
	}

	return dependents
}

// stepRecordToInfo converts an internal stepRecord to a public StepInfo.
func stepRecordToInfo(rec *stepRecord) StepInfo {
	return StepInfo{
		Name:         rec.name,
		Type:         rec.stepType,
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

func cmpStrings(a, b string) int {
	if a < b {
		return -1
	}

	if a > b {
		return 1
	}

	return 0
}

// buildReportFromCore assembles a WorkflowReport from core data, deriving all
// denormalized aggregate fields. This is the single construction path.
func buildReportFromCore(
	version, workflowID string,
	exportedAt time.Time,
	droppedEventCount int64,
	events []Event,
	steps []StepInfo,
) WorkflowReport {
	report := WorkflowReport{ //nolint:exhaustruct
		Version:           version,
		WorkflowID:        workflowID,
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
		}

		report.TotalDurationMs += step.Duration()
	}

	report.WorkflowSucceeded = report.FailedCount == 0 &&
		report.CanceledCount == 0
}
