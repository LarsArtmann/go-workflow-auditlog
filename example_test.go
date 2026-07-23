package auditlog_test

import (
	"bytes"
	"fmt"
	"time"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// ExampleNDJSONStreamer demonstrates real-time streaming of workflow events
// as NDJSON. Events are written the moment they are captured — no need to
// wait for the workflow to finish.
func ExampleNDJSONStreamer() {
	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf)

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "fetch"},
	})

	streamer.OnEvent(auditlog.Event{
		Sequence:  2,
		EventType: auditlog.EventTypeAttemptEnd,
		Phase:     auditlog.PhaseAfter,
		StepRef:   auditlog.StepRef{Name: "fetch"},
		Status:    auditlog.StepStatusSucceeded,
	})

	_ = streamer.Close()

	events, _ := auditlog.ReadEvents(&buf)

	fmt.Printf("streamed %d events, first: %s\n", len(events), events[0].Name)

	// Output: streamed 2 events, first: fetch
}

// ExampleReplayEvents demonstrates reconstructing a report from a flat NDJSON
// event stream. This is useful when you have exported events and want to
// analyze them without the original Auditor instance.
func ExampleReplayEvents() {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	events := []auditlog.Event{
		{
			Sequence:  1,
			Timestamp: now,
			EventType: auditlog.EventTypeAttemptStart,
			Phase:     auditlog.PhaseBefore,
			StepRef:   auditlog.StepRef{Name: "fetch"},
			Attempt:   1,
		},
		{
			Sequence:  2,
			Timestamp: now.Add(10 * time.Millisecond),
			EventType: auditlog.EventTypeAttemptEnd,
			Phase:     auditlog.PhaseAfter,
			StepRef:   auditlog.StepRef{Name: "fetch"},
			Attempt:   1,
			Status:    auditlog.StepStatusSucceeded,
		},
	}

	report, _ := auditlog.ReplayEvents(events)

	fmt.Printf("reconstructed: %d steps, %d events\n", report.StepCount, report.EventCount)

	// Output: reconstructed: 1 steps, 2 events
}

// ExampleWorkflowReport_Summary demonstrates the one-line summary of a report,
// which uses wall-clock duration and includes the failure reason when the
// workflow did not succeed.
func ExampleWorkflowReport_Summary() {
	report := auditlog.WorkflowReport{
		WorkflowID:          "data-pipeline",
		WallClockDurationMs: 28500,
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "fetch"}, Status: auditlog.StepStatusSucceeded},
		},
	}

	finalizeForExample(&report)
	fmt.Println(report.Summary())

	// Output: data-pipeline: 1 steps (1 ok, 0 failed, 0 skipped) in 28500.0ms
}

// ExampleWorkflowReport_CriticalPath demonstrates finding the bottleneck
// dependency chain — the longest path through the DAG that determines the
// minimum possible wall-clock time.
func ExampleWorkflowReport_CriticalPath() {
	dur := func(ms float64) *float64 { return &ms }

	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "fetch"}, DurationMs: dur(10)},
			{
				StepRef:      auditlog.StepRef{Name: "transform"},
				DurationMs:   dur(50),
				Dependencies: []auditlog.StepRef{{Name: "fetch"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "save"},
				DurationMs:   dur(5),
				Dependencies: []auditlog.StepRef{{Name: "transform"}},
			},
		},
	}

	for _, step := range report.CriticalPath() {
		fmt.Println(step.Name)
	}

	// Output:
	// fetch
	// transform
	// save
}

// ExampleReadEvents demonstrates reading NDJSON events from a reader — the
// inverse of WriteNDJSON. This lets you load exported event streams for
// analysis or replay.
func ExampleReadEvents() {
	ndjson := `{"sequence":1,"timestamp":"2026-01-01T00:00:00Z","event_type":"attempt_start","phase":"before","step_name":"fetch","attempt":1}` + "\n" +
		`{"sequence":2,"timestamp":"2026-01-01T00:00:01Z","event_type":"attempt_end","phase":"after","step_name":"fetch","attempt":1,"status":"succeeded"}` + "\n"

	events, _ := auditlog.ReadEvents(bytes.NewReader([]byte(ndjson)))

	fmt.Printf("read %d events\n", len(events))

	// Output: read 2 events
}

// finalizeForExample populates denormalized fields so Summary() works in
// examples without running a full Auditor lifecycle.
func finalizeForExample(r *auditlog.WorkflowReport) {
	r.EventCount = len(r.Events)
	r.StepCount = len(r.Steps)

	for _, s := range r.Steps {
		if s.Status == auditlog.StepStatusSucceeded {
			r.SucceededCount++
		}
	}

	r.WorkflowSucceeded = r.FailedCount == 0 && r.PendingCount == 0 && r.RunningCount == 0
}
