package auditlog_test

import (
	"bytes"
	"fmt"

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
