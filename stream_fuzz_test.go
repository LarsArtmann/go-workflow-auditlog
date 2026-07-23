package auditlog_test

import (
	"bytes"
	"testing"
	"time"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// FuzzNDJSONStreamer fuzzes the NDJSON streamer with adversarial event
// payloads. The streamer must never panic and must either produce valid
// output or record an error via Err().
func FuzzNDJSONStreamer(f *testing.F) {
	seeds := []auditlog.Event{
		// Minimal valid event.
		{
			Sequence:  1,
			EventType: auditlog.EventTypeAttemptStart,
			Phase:     auditlog.PhaseBefore,
			StepRef:   auditlog.StepRef{Name: "s"},
		},
		// Event with all fields.
		{
			Sequence:  1,
			EventType: auditlog.EventTypeAttemptEnd,
			Phase:     auditlog.PhaseAfter,
			StepRef:   auditlog.StepRef{Name: "step", StepType: "MyStep"},
			Timestamp: time.Now(),
			Attempt:   3,
			Status:    auditlog.StepStatusFailed,
		},
		// Empty step name.
		{
			Sequence:  1,
			EventType: auditlog.EventTypeAttemptStart,
			Phase:     auditlog.PhaseBefore,
			StepRef:   auditlog.StepRef{Name: ""},
		},
		// Event with special characters in name.
		{
			Sequence:  1,
			EventType: auditlog.EventTypeAttemptStart,
			Phase:     auditlog.PhaseBefore,
			StepRef:   auditlog.StepRef{Name: "<script>alert('xss')</script>"},
		},
		// Event with unicode name.
		{
			Sequence:  1,
			EventType: auditlog.EventTypeAttemptStart,
			Phase:     auditlog.PhaseBefore,
			StepRef:   auditlog.StepRef{Name: "ステップ"},
		},
		// Very long step name.
		{
			Sequence:  1,
			EventType: auditlog.EventTypeAttemptStart,
			Phase:     auditlog.PhaseBefore,
			StepRef:   auditlog.StepRef{Name: string(make([]byte, 10000))},
		},
		// Event with unusual but valid fields.
		{
			Sequence:  -1,
			EventType: auditlog.EventTypeAttemptStart,
			Phase:     auditlog.PhaseBefore,
			StepRef:   auditlog.StepRef{Name: "neg-seq"},
		},
	}

	for _, seed := range seeds {
		f.Add(seed.Sequence, string(seed.EventType), string(seed.Phase), seed.Name, seed.StepType)
	}

	f.Fuzz(func(t *testing.T, seq int, evtType, phase, name, stepType string) {
		var buf bytes.Buffer

		streamer := auditlog.NewNDJSONStreamer(&buf)

		// Construct an event from fuzz inputs. Use valid types when possible
		// so the output is parseable; otherwise verify no panic.
		evt := auditlog.Event{
			Sequence:  seq,
			EventType: auditlog.EventType(evtType),
			Phase:     auditlog.Phase(phase),
			StepRef:   auditlog.StepRef{Name: name, StepType: stepType},
		}

		if evt.EventType == "" {
			evt.EventType = auditlog.EventTypeAttemptStart
		}

		if evt.Phase == "" {
			evt.Phase = auditlog.PhaseBefore
		}

		streamer.OnEvent(evt)

		err := streamer.Flush()
		if err != nil {
			// Error is acceptable — verify Err() captures it.
			if streamer.Err() == nil {
				t.Error("Flush returned error but Err() is nil")
			}

			return
		}

		// On success, verify output is valid NDJSON. If the fuzz-provided
		// event_type or phase is non-standard, ReadEvents will reject it on
		// ingest — that's the reader's validation job, not a streamer bug.
		events, err := auditlog.ReadEvents(&buf)
		if err != nil {
			return
		}

		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}

		// The round-tripped event should preserve the name.
		if events[0].Name != name {
			t.Errorf("name mismatch: got %q, want %q", events[0].Name, name)
		}

		// Verify the sequence survived.
		if events[0].Sequence != seq {
			t.Errorf("sequence mismatch: got %d, want %d", events[0].Sequence, seq)
		}
	})
}

// FuzzReplayEvents fuzzes the replay path: stream events through NDJSON,
// read them back, and replay into a report. The replay must never panic and
// must produce a consistent report or an error.
func FuzzReplayEvents(f *testing.F) {
	seeds := [][]auditlog.Event{
		// Single start+end pair.
		{
			{
				Sequence:  1,
				Timestamp: time.Now(),
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
				StepRef:   auditlog.StepRef{Name: "a"},
				Attempt:   1,
			},
			{
				Sequence:  2,
				Timestamp: time.Now(),
				EventType: auditlog.EventTypeAttemptEnd,
				Phase:     auditlog.PhaseAfter,
				StepRef:   auditlog.StepRef{Name: "a"},
				Attempt:   1,
				Status:    auditlog.StepStatusSucceeded,
			},
		},
		// Empty events.
		{},
		// Single event (no matching end).
		{
			{
				Sequence:  1,
				Timestamp: time.Now(),
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
				StepRef:   auditlog.StepRef{Name: "lone"},
				Attempt:   1,
			},
		},
	}

	for _, seed := range seeds {
		for _, evt := range seed {
			f.Add(evt.Sequence, evt.Name)
		}
	}

	f.Fuzz(func(t *testing.T, seq int, name string) {
		// Build a minimal event pair from fuzz inputs.
		now := time.Now()

		events := []auditlog.Event{
			{
				Sequence:  seq,
				Timestamp: now,
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
				StepRef:   auditlog.StepRef{Name: name},
				Attempt:   1,
			},
			{
				Sequence:  seq + 1,
				Timestamp: now.Add(time.Millisecond),
				EventType: auditlog.EventTypeAttemptEnd,
				Phase:     auditlog.PhaseAfter,
				StepRef:   auditlog.StepRef{Name: name},
				Attempt:   1,
				Status:    auditlog.StepStatusSucceeded,
			},
		}

		// Stream through NDJSON.
		var buf bytes.Buffer

		streamer := auditlog.NewNDJSONStreamer(&buf)

		for _, evt := range events {
			streamer.OnEvent(evt)
		}

		err := streamer.Flush()
		if err != nil {
			return // encoding error is acceptable
		}

		// Read back.
		readEvents, err := auditlog.ReadEvents(&buf)
		if err != nil {
			return // unreadable output is acceptable for adversarial input
		}

		if len(readEvents) == 0 {
			return
		}

		// Replay into a report.
		report, err := auditlog.ReplayEvents(readEvents)
		if err != nil {
			return // replay error is acceptable
		}

		// Verify the report is internally consistent.
		err = report.Validate()
		if err != nil {
			t.Errorf("replayed report failed validation: %v", err)
		}
	})
}
