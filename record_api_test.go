package auditlog_test

import (
	"bytes"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// PD2/M7: hosts must be able to inject synthetic events (the pre-DAG window:
// doctor preflight, cache warmup) into the exported audit stream. Record
// stamps run_id, sequence, and timestamp when unset so injected events
// interleave with captured ones, and the event round-trips through NDJSON.
func TestAuditorRecord_InjectsSyntheticEvent(t *testing.T) {
	auditor, err := auditlog.New(auditlog.Config{WorkflowID: "wf-record-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	before := auditlog.Event{
		EventType: auditlog.EventTypeSystem,
		Phase:     auditlog.PhaseBefore,
	}
	auditor.Record(before)

	events := auditor.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 injected event, got %d", len(events))
	}

	evt := events[0]
	if evt.EventType != auditlog.EventTypeSystem {
		t.Fatalf("event type = %q", evt.EventType)
	}

	if evt.Sequence == 0 {
		t.Fatal("sequence must be stamped on injected events")
	}

	if evt.RunID == "" {
		t.Fatal("run_id must be stamped on injected events")
	}

	if evt.Timestamp.IsZero() {
		t.Fatal("timestamp must be stamped on injected events")
	}

	var buf bytes.Buffer
	if err := auditor.WriteNDJSON(&buf); err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}

	roundTripped, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(roundTripped) != 1 || roundTripped[0].EventType != auditlog.EventTypeSystem {
		t.Fatalf("round trip lost the injected event: %+v", roundTripped)
	}
}
