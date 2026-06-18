package auditlog

import (
	"errors"
	"fmt"
	"time"
)

// ErrReplayNoEvents is returned when ReplayEvents receives zero events.
var ErrReplayNoEvents = errors.New("no events to replay")

// replayStep is the accumulator for a single step during replay.
type replayStep struct {
	StepRef

	attemptCount int
	startedAt    *time.Time
	finishedAt   *time.Time
	durationMs   *float64
	attemptErr   *string
	status       StepStatus
}

// ReplayEvents reconstructs a WorkflowReport from a flat event stream.
//
// This is the inverse of ExportEventsToNDJSON: write events to NDJSON, then
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

	steps := make(map[string]*replayStep)

	for _, evt := range events {
		step := getOrCreateReplayStep(steps, evt.Name, evt.StepType)

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

	stepInfos := make([]StepInfo, 0, len(steps))
	for _, rs := range steps {
		stepInfos = append(stepInfos, StepInfo{
			StepRef:      rs.StepRef,
			Status:       rs.status,
			AttemptCount: rs.attemptCount,
			StartedAt:    rs.startedAt,
			FinishedAt:   rs.finishedAt,
			DurationMs:   rs.durationMs,
			Error:        rs.attemptErr,
		})
	}

	report := buildReportFromCore(
		SchemaVersion,
		"", // workflowID not available from events alone
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

func getOrCreateReplayStep(steps map[string]*replayStep, name, stepType string) *replayStep {
	if step, ok := steps[name]; ok {
		return step
	}

	step := &replayStep{
		StepRef: StepRef{Name: name, StepType: stepType},
		status:  StepStatusPending,
	}
	steps[name] = step

	return step
}
