package auditlog_test

import (
	"errors"
	"strings"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// TestStreamEvents_DeliversInOrder verifies that StreamEvents invokes the
// callback once per parsed event, in source order, with monotonically
// increasing lineNum.
func TestStreamEvents_DeliversInOrder(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`{"sequence":1,"timestamp":"2026-01-01T00:00:00Z","event_type":"attempt_start","phase":"before","step_name":"s1"}`,
		`{"sequence":2,"timestamp":"2026-01-01T00:00:01Z","event_type":"attempt_end","phase":"after","step_name":"s1"}`,
		`{"sequence":3,"timestamp":"2026-01-01T00:00:02Z","event_type":"attempt_start","phase":"before","step_name":"s2"}`,
	}, "\n") + "\n"

	var delivered []auditlog.Event

	var lineNums []int

	err := auditlog.StreamEvents(
		strings.NewReader(input),
		nil,
		func(lineNum int, evt auditlog.Event) error {
			lineNums = append(lineNums, lineNum)

			delivered = append(delivered, evt)

			return nil
		},
	)
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}

	if len(delivered) != 3 {
		t.Fatalf("expected 3 events delivered, got %d", len(delivered))
	}

	wantSeqs := []int{1, 2, 3}

	for i, evt := range delivered {
		if evt.Sequence != wantSeqs[i] {
			t.Errorf("event %d: sequence=%d, want %d", i, evt.Sequence, wantSeqs[i])
		}

		if lineNums[i] != i+1 {
			t.Errorf("event %d: lineNum=%d, want %d", i, lineNums[i], i+1)
		}
	}
}

// TestStreamEvents_EmptyInput verifies ErrEmpty for zero bytes.
func TestStreamEvents_EmptyInput(t *testing.T) {
	t.Parallel()

	var called int

	err := auditlog.StreamEvents(
		strings.NewReader(""),
		nil,
		func(int, auditlog.Event) error {
			called++

			return nil
		},
	)
	if !errors.Is(err, auditlog.ErrEmpty) {
		t.Errorf("expected ErrEmpty, got %v", err)
	}

	if called != 0 {
		t.Errorf("callback should not be called for empty input; got %d calls", called)
	}
}

// TestStreamEvents_AllBlank verifies ErrNoEvents when input is whitespace only.
func TestStreamEvents_AllBlank(t *testing.T) {
	t.Parallel()

	var called int

	err := auditlog.StreamEvents(
		strings.NewReader("\n  \n\t\n"),
		nil,
		func(int, auditlog.Event) error {
			called++

			return nil
		},
	)
	if !errors.Is(err, auditlog.ErrNoEvents) {
		t.Errorf("expected ErrNoEvents, got %v", err)
	}

	if called != 0 {
		t.Errorf("callback should not be called for blank-only input; got %d calls", called)
	}
}

// TestStreamEvents_NilCallback verifies the nil-callback guard.
func TestStreamEvents_NilCallback(t *testing.T) {
	t.Parallel()

	err := auditlog.StreamEvents(strings.NewReader(""), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil callback")
	}
}

// TestStreamEvents_CallbackError verifies that a non-nil callback error stops
// iteration and is wrapped with the line number.
func TestStreamEvents_CallbackError(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`{"sequence":1,"timestamp":"2026-01-01T00:00:00Z","event_type":"attempt_start","phase":"before","step_name":"s1"}`,
		`{"sequence":2,"timestamp":"2026-01-01T00:00:01Z","event_type":"attempt_end","phase":"after","step_name":"s1"}`,
		`{"sequence":3,"timestamp":"2026-01-01T00:00:02Z","event_type":"attempt_start","phase":"before","step_name":"s2"}`,
	}, "\n") + "\n"

	sentinel := errors.New("downstream failure")

	var called int

	err := auditlog.StreamEvents(
		strings.NewReader(input),
		nil,
		func(int, auditlog.Event) error {
			called++

			if called == 2 {
				return sentinel
			}

			return nil
		},
	)
	if err == nil {
		t.Fatal("expected error from callback")
	}

	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}

	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error should mention line 2; got %v", err)
	}

	if called != 2 {
		t.Errorf("callback should stop after error; got %d calls", called)
	}
}

// TestStreamEvents_InvokesValidate verifies that the optional validate
// function is called per line (same semantics as ReadEvents).
func TestStreamEvents_InvokesValidate(t *testing.T) {
	t.Parallel()

	input := `{"sequence":1,"timestamp":"2026-01-01T00:00:00Z","event_type":"bogus","phase":"before","step_name":"s1"}` + "\n"

	var validated int

	err := auditlog.StreamEvents(
		strings.NewReader(input),
		func(int, auditlog.Event) error {
			validated++

			return errors.New("rejected")
		},
		func(int, auditlog.Event) error {
			t.Fatal("callback should not run after validate rejects")

			return nil
		},
	)
	if err == nil {
		t.Fatal("expected validate rejection error")
	}

	if !strings.Contains(err.Error(), "rejected") {
		t.Errorf("error should wrap validate error; got %v", err)
	}

	if validated != 1 {
		t.Errorf("validate should be called once; got %d", validated)
	}
}

// TestStreamEvents_OversizedLine verifies that a line exceeding
// ndjson.MaxLineBytes (1 MB) is rejected with ErrOversizedLine.
func TestStreamEvents_OversizedLine(t *testing.T) {
	t.Parallel()

	// Build a valid JSON event whose serialized form exceeds 1 MB
	// by embedding a huge step name.
	pad := strings.Repeat("x", 1<<20) // 1 MiB of padding
	huge := `{"sequence":1,"timestamp":"2026-01-01T00:00:00Z","event_type":"attempt_start","phase":"before","step_name":"` + pad + `"}` + "\n"

	err := auditlog.StreamEvents(
		strings.NewReader(huge),
		nil,
		func(int, auditlog.Event) error { return nil },
	)
	if !errors.Is(err, auditlog.ErrOversizedLine) {
		t.Errorf("expected ErrOversizedLine, got %v", err)
	}
}

// TestStreamEvents_AllLinesFailJSON verifies that non-blank input where every
// line fails JSON parsing returns a descriptive error (not ErrNoEvents).
func TestStreamEvents_AllLinesFailJSON(t *testing.T) {
	t.Parallel()

	input := "not json at all\n{broken\n  \n"

	err := auditlog.StreamEvents(
		strings.NewReader(input),
		nil,
		func(int, auditlog.Event) error { return nil },
	)
	if err == nil {
		t.Fatal("expected error for all-invalid JSON lines, got nil")
	}

	if errors.Is(err, auditlog.ErrNoEvents) {
		t.Errorf("should not be ErrNoEvents for parse failure, got %v", err)
	}

	if !strings.Contains(err.Error(), "ndjson line") {
		t.Errorf("error should contain line reference, got %v", err)
	}
}
