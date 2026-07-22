package auditlog_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	flow "github.com/Azure/go-workflow"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

func TestNDJSONStreamer_BasicRoundTrip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf)

	originalEvents := []auditlog.Event{
		{
			Sequence: 1, EventType: auditlog.EventTypeAttemptStart, Phase: auditlog.PhaseBefore,
			StepRef: auditlog.StepRef{Name: "step-a"},
		},
		{
			Sequence: 2, EventType: auditlog.EventTypeAttemptEnd, Phase: auditlog.PhaseAfter,
			StepRef: auditlog.StepRef{Name: "step-a"}, Status: auditlog.StepStatusSucceeded,
		},
	}

	for _, evt := range originalEvents {
		streamer.OnEvent(evt)
	}

	err := streamer.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	events, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != len(originalEvents) {
		t.Fatalf("expected %d events, got %d", len(originalEvents), len(events))
	}

	for i, evt := range events {
		if evt.Sequence != originalEvents[i].Sequence {
			t.Errorf("event %d: expected seq %d, got %d", i, originalEvents[i].Sequence, evt.Sequence)
		}

		if evt.EventType != originalEvents[i].EventType {
			t.Errorf("event %d: expected type %s, got %s", i, originalEvents[i].EventType, evt.EventType)
		}

		if evt.Name != originalEvents[i].Name {
			t.Errorf("event %d: expected name %q, got %q", i, originalEvents[i].Name, evt.Name)
		}
	}
}

func TestNDJSONStreamer_EmptyFlush(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf)

	err := streamer.Flush()
	if err != nil {
		t.Fatalf("Flush on empty streamer: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %d bytes", buf.Len())
	}
}

func TestNDJSONStreamer_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	const goroutines = 16

	const eventsPerGoroutine = 50

	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf)

	var wg sync.WaitGroup

	for g := range goroutines {
		wg.Add(1)

		go func(groupID int) {
			defer wg.Done()

			for i := range eventsPerGoroutine {
				streamer.OnEvent(auditlog.Event{
					Sequence:  groupID*eventsPerGoroutine + i + 1,
					EventType: auditlog.EventTypeAttemptStart,
					Phase:     auditlog.PhaseBefore,
					StepRef:   auditlog.StepRef{Name: "concurrent"},
				})
			}
		}(g)
	}

	wg.Wait()

	err := streamer.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	events, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents failed (corrupted output): %v", err)
	}

	expected := goroutines * eventsPerGoroutine
	if len(events) != expected {
		t.Fatalf("expected %d events, got %d (data loss or corruption)", expected, len(events))
	}
}

func TestNDJSONStreamer_AutoFlush(t *testing.T) {
	t.Parallel()

	tracker := newWriteTracker()

	streamer := auditlog.NewNDJSONStreamer(tracker, auditlog.WithAutoFlush())

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "auto"},
	})

	// With auto-flush, data must be visible immediately (no Flush call).
	if tracker.writtenBytes() == 0 {
		t.Fatal("expected data written to underlying writer immediately with WithAutoFlush")
	}
}

func TestNDJSONStreamer_BufferedThenFlush(t *testing.T) {
	t.Parallel()

	tracker := newWriteTracker()

	streamer := auditlog.NewNDJSONStreamer(tracker) // no auto-flush

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "buffered"},
	})

	// Without auto-flush, data may still be buffered (event is small).
	// After Flush, all data must be in the underlying writer.
	err := streamer.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if tracker.writtenBytes() == 0 {
		t.Fatal("expected data in underlying writer after Flush")
	}

	events, err := auditlog.ReadEvents(strings.NewReader(tracker.String()))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestNDJSONStreamer_ErrorHandling(t *testing.T) {
	t.Parallel()

	streamer := auditlog.NewNDJSONStreamer(&errorWriter{})

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "fail"},
	})

	err := streamer.Flush()
	if err == nil {
		t.Fatal("expected error from Flush with failing writer")
	}

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("expected ErrExportWriteFailed, got %v", err)
	}
}

func TestNDJSONStreamer_OnEventAfterError(t *testing.T) {
	t.Parallel()

	// WithAutoFlush forces immediate writes so the error surfaces on the
	// first OnEvent call, not deferred to Flush.
	streamer := auditlog.NewNDJSONStreamer(&errorWriter{}, auditlog.WithAutoFlush())

	// First event triggers the write error.
	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "first"},
	})

	firstErr := streamer.Err()
	if firstErr == nil {
		t.Fatal("expected error after first event")
	}

	// Second event must be silently dropped (no panic).
	streamer.OnEvent(auditlog.Event{
		Sequence:  2,
		EventType: auditlog.EventTypeAttemptEnd,
		Phase:     auditlog.PhaseAfter,
		StepRef:   auditlog.StepRef{Name: "second"},
	})

	// Err() must still report the same family of error.
	if !errors.Is(streamer.Err(), auditlog.ErrExportWriteFailed) {
		t.Errorf("expected ErrExportWriteFailed after subsequent OnEvent, got %v", streamer.Err())
	}
}

func TestNDJSONStreamer_Close(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "close-test.ndjson")

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	streamer := auditlog.NewNDJSONStreamer(file)

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "close-test"},
	})

	err = streamer.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// File must be closed and readable.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	events, err := auditlog.ReadEvents(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event after Close, got %d", len(events))
	}
}

func TestNDJSONStreamer_DoubleClose(t *testing.T) {
	t.Parallel()

	streamer := auditlog.NewNDJSONStreamer(&bytes.Buffer{})

	err := streamer.Close()
	if err != nil {
		t.Fatalf("first Close: %v", err)
	}

	err = streamer.Close()
	if err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestNDJSONStreamer_OnEventAfterClose(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf)

	err := streamer.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Events after close must be silently dropped.
	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "after-close"},
	})

	if buf.Len() != 0 {
		t.Errorf("expected no output after Close, got %d bytes", buf.Len())
	}
}

func TestNDJSONStreamer_CreateFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "create-stream.ndjson")

	streamer, err := auditlog.CreateNDJSONStreamer(path)
	if err != nil {
		t.Fatalf("CreateNDJSONStreamer: %v", err)
	}

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "create"},
	})

	err = streamer.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	events, err := auditlog.ReadEvents(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Name != "create" {
		t.Errorf("expected name %q, got %q", "create", events[0].Name)
	}
}

func TestNDJSONStreamer_WorkflowIntegration(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf)

	a := testhelpers.MustNew(t, auditlog.Config{
		Enabled: true,
		OnEvent: streamer.OnEvent,
	})

	w := &flow.Workflow{}

	s1 := testhelpers.NewSucceed("stream-a")
	s2 := testhelpers.NewFail("stream-b", "boom")
	testhelpers.AddParallelSteps(w, s1, s2)
	testhelpers.RunWorkflow(t, a, w)

	err := streamer.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	streamedEvents, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	recorderEvents := a.Events()
	if len(streamedEvents) != len(recorderEvents) {
		t.Fatalf("streamed %d events, recorder has %d", len(streamedEvents), len(recorderEvents))
	}

	// Verify every recorder event appears in the stream (order may differ).
	streamedBySeq := make(map[int]auditlog.Event, len(streamedEvents))
	for _, evt := range streamedEvents {
		streamedBySeq[evt.Sequence] = evt
	}

	for _, expected := range recorderEvents {
		got, ok := streamedBySeq[expected.Sequence]
		if !ok {
			t.Errorf("event seq %d missing from stream", expected.Sequence)

			continue
		}

		if got.Name != expected.Name {
			t.Errorf("event seq %d: name %q != %q", expected.Sequence, got.Name, expected.Name)
		}

		if got.EventType != expected.EventType {
			t.Errorf("event seq %d: type %s != %s", expected.Sequence, got.EventType, expected.EventType)
		}

		if got.RunID != expected.RunID {
			t.Errorf("event seq %d: run_id %q != %q", expected.Sequence, got.RunID, expected.RunID)
		}
	}
}

func TestNDJSONStreamer_CreateFileError(t *testing.T) {
	t.Parallel()

	// Directory does not exist.
	_, err := auditlog.CreateNDJSONStreamer("/nonexistent/path/that/does/not/exist/file.ndjson")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("expected ErrExportWriteFailed, got %v", err)
	}
}

func TestNDJSONStreamer_NilErr(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf)

	err := streamer.Err()
	if err != nil {
		t.Errorf("expected nil Err on fresh streamer, got %v", err)
	}

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "nil-err"},
	})

	err = streamer.Err()
	if err != nil {
		t.Errorf("expected nil Err after successful write, got %v", err)
	}
}

func TestNDJSONStreamer_AutoFlushWorkflowIntegration(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "autoflush.ndjson")

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = file.Close() }()

	streamer := auditlog.NewNDJSONStreamer(file, auditlog.WithAutoFlush())

	a := testhelpers.MustNew(t, auditlog.Config{
		Enabled: true,
		OnEvent: streamer.OnEvent,
	})

	w := &flow.Workflow{}
	w.Add(flow.Step(testhelpers.NewSucceed("autoflush-step")))
	testhelpers.RunWorkflow(t, a, w)

	// Without explicit Flush, auto-flush should have written all events.
	_ = file.Sync()

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}

	events, readErr := auditlog.ReadEvents(bytes.NewReader(data))
	if readErr != nil {
		t.Fatalf("ReadEvents: %v", readErr)
	}

	if len(events) == 0 {
		t.Fatal("expected events in file from auto-flushed streamer")
	}

	recorderEvents := a.Events()
	if len(events) != len(recorderEvents) {
		t.Fatalf("expected %d events, got %d", len(recorderEvents), len(events))
	}
}

// --- Helpers ---

// errorWriter is an io.Writer that always fails.
type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

// writeTracker is a thread-safe io.Writer that records all bytes written.
type writeTracker struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newWriteTracker() *writeTracker { return &writeTracker{} }

func (w *writeTracker) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.Write(p)
}

func (w *writeTracker) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.String()
}

func (w *writeTracker) writtenBytes() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.Len()
}

// Ensure the streamer is used as a context-compatible OnEvent in a real
// workflow run — exercises the Attach → Do → Snapshot → Flush lifecycle.
func TestNDJSONStreamer_FullLifecycleExample(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf)

	a := testhelpers.MustNew(t, auditlog.Config{
		Enabled: true,
		OnEvent: streamer.OnEvent,
	})

	w := &flow.Workflow{}
	w.Add(flow.Step(testhelpers.NewSucceed("lifecycle-step")))

	a.Attach(w)
	_ = w.Do(context.TODO())
	a.Snapshot(w)

	err := streamer.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("expected non-empty NDJSON output from full lifecycle")
	}

	events, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	testhelpers.AssertEventCount(t, a.Report(), len(events))
}
