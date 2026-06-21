package auditlog_test

import (
	"strings"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// FuzzReadEvents fuzzes the NDJSON reader with arbitrary input. The reader
// must never panic and must always return either valid events or a
// non-nil error. It must reject lines with unknown event_type or phase values.
func FuzzReadEvents(f *testing.F) {
	seeds := []string{
		// Valid single event.
		`{"sequence":1,"timestamp":"2026-01-01T00:00:00Z","event_type":"attempt_start","phase":"before","step_name":"step1"}`,
		// Valid two events.
		`{"sequence":1,"timestamp":"2026-01-01T00:00:00Z","event_type":"attempt_start","phase":"before","step_name":"s1"}` + "\n" +
			`{"sequence":2,"timestamp":"2026-01-01T00:00:01Z","event_type":"attempt_end","phase":"after","step_name":"s1"}`,
		// Empty input.
		"",
		// Only whitespace.
		"   \n\t\n  \n",
		// Invalid JSON.
		"not json at all",
		"{broken",
		// Unknown event_type.
		`{"sequence":1,"event_type":"bogus_type","phase":"before","step_name":"s"}`,
		// Unknown phase.
		`{"sequence":1,"event_type":"attempt_start","phase":"bogus_phase","step_name":"s"}`,
		// Line with extra whitespace around valid JSON.
		"  " + `{"sequence":1,"event_type":"attempt_end","phase":"after","step_name":"s"}` + "  \n",
		// Very long line (should be rejected if over 1MB).
		strings.Repeat(`{"sequence":1,"event_type":"attempt_start","phase":"before","step_name":"s"}`, 100000),
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		reader := strings.NewReader(input)

		events, err := auditlog.ReadEvents(reader)
		if err != nil {
			// On error, events should be nil or empty — never partially populated.
			if len(events) > 0 {
				t.Errorf("ReadEvents returned %d events alongside error %v", len(events), err)
			}

			return
		}

		// On success, every returned event must have a valid event_type and phase.
		for i, evt := range events {
			if evt.EventType != "" && !evt.EventType.IsKnown() {
				t.Errorf("event %d has unknown event_type %q", i, evt.EventType)
			}

			if evt.Phase != "" && !evt.Phase.IsKnown() {
				t.Errorf("event %d has unknown phase %q", i, evt.Phase)
			}
		}
	})
}
