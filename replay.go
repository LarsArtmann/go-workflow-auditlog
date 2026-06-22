package auditlog

import (
	"errors"
	"fmt"
	"time"
)

// ErrReplayNoEvents is returned when ReplayEvents receives zero events.
var ErrReplayNoEvents = errors.New("no events to replay")

// ReplayEvents reconstructs a WorkflowReport from a flat event stream.
//
// This is the inverse of ExportNDJSON: write events to NDJSON, then
// later read them back and reconstruct the report for offline analysis.
//
// Limitations:
//   - Dependencies/dependents are not inferred (events carry no DAG edges).
//     Use Snapshot-captured reports for full DAG info.
//   - MaxAttempts/HasRetry/HasTimeout are not available from events alone.
//   - The report has Reconstructed=true.
func ReplayEvents(events []Event) (WorkflowReport, error) {
	if len(events) == 0 {
		return WorkflowReport{}, ErrReplayNoEvents
	}

	stepInfos := replayStepsFromEvents(events)

	report := buildReportFromCore(
		SchemaVersion,
		"",              // workflowID not available from events alone
		events[0].RunID, // runID is stamped on every event during capture
		time.Now(),
		0, // no dropped events in replay
		events,
		stepInfos,
	)
	report.Reconstructed = true

	err := report.Validate()
	if err != nil {
		return report, fmt.Errorf("replayed report failed validation: %w", err)
	}

	return report, nil
}

// replayStepsFromEvents folds a flat event stream into sorted StepInfo records
// with stable 1-based StepIDs.
func replayStepsFromEvents(events []Event) []StepInfo {
	steps := make(map[string]*stepCore)

	for _, evt := range events {
		step := getOrCreateReplayStep(steps, evt.Name, evt.StepType)
		replayApplyEvent(step, evt)
	}

	stepInfos := make([]StepInfo, 0, len(steps))
	for _, rs := range steps {
		stepInfos = append(stepInfos, rs.toStepInfo())
	}

	// Sort by name for deterministic output, then assign stable 1-based IDs.
	sortStepsByName(stepInfos)

	for i := range stepInfos {
		stepInfos[i].StepID = i + 1
	}

	return stepInfos
}

// replayApplyEvent updates a stepCore accumulator from a single event.
func replayApplyEvent(step *stepCore, evt Event) {
	switch {
	case evt.IsAttemptStart():
		if step.startedAt == nil {
			started := evt.Timestamp
			step.startedAt = &started
		}

		step.attemptCount = max(step.attemptCount, evt.Attempt)

	case evt.IsAttemptEnd():
		finished := evt.Timestamp
		step.finishedAt = &finished
		step.durationMs = evt.DurationMs
		step.status = evt.Status

		if evt.Error != nil {
			errStr := *evt.Error
			step.attemptErr = &errStr
		}
	}
}

func getOrCreateReplayStep(steps map[string]*stepCore, name, stepType string) *stepCore {
	if step, ok := steps[name]; ok {
		return step
	}

	step := &stepCore{
		StepRef: StepRef{Name: name, StepType: stepType},
		status:  StepStatusPending,
	}
	steps[name] = step

	return step
}
