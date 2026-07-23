package auditlog_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

	streamer := auditlog.NewNDJSONStreamer(&testhelpers.FailingWriter{})

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
	streamer := auditlog.NewNDJSONStreamer(&testhelpers.FailingWriter{}, auditlog.WithAutoFlush())

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

func TestNDJSONStreamer_WithBufferSize(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf, auditlog.WithBufferSize(128))

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "small-buf"},
	})

	err := streamer.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	events, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Name != "small-buf" {
		t.Errorf("expected name %q, got %q", "small-buf", events[0].Name)
	}
}

// WithBufferSize(1) forces bufio to bypass its buffer and write directly to
// the underlying writer on every call. Combined with FailingWriter this
// triggers the encode-error branch in OnEvent (which is unreachable with the
// default 64 KB buffer because bufio swallows the write).
func TestNDJSONStreamer_EncodeError(t *testing.T) {
	t.Parallel()

	streamer := auditlog.NewNDJSONStreamer(
		&testhelpers.FailingWriter{},
		auditlog.WithBufferSize(1),
	)

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "encode-fail"},
	})

	if !errors.Is(streamer.Err(), auditlog.ErrRenderFailed) {
		t.Errorf("expected ErrRenderFailed from encode error, got %v", streamer.Err())
	}
}

func TestNDJSONStreamer_FlushReturnsExistingError(t *testing.T) {
	t.Parallel()

	streamer := auditlog.NewNDJSONStreamer(
		&testhelpers.FailingWriter{},
		auditlog.WithBufferSize(1),
	)

	// Trigger an encode error (buffer size 1 bypasses bufio).
	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "trigger"},
	})

	firstErr := streamer.Err()
	if firstErr == nil {
		t.Fatal("expected error after OnEvent with failing writer")
	}

	// Flush must short-circuit and return the existing error.
	flushErr := streamer.Flush()
	if flushErr == nil {
		t.Fatal("expected error from Flush after prior error")
	}

	if !errors.Is(flushErr, auditlog.ErrRenderFailed) {
		t.Errorf("expected ErrRenderFailed from Flush, got %v", flushErr)
	}
}

func TestNDJSONStreamer_CloseError(t *testing.T) {
	t.Parallel()

	w := &closeFailWriter{}

	streamer := auditlog.NewNDJSONStreamer(w)

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "close-fail"},
	})

	err := streamer.Close()
	if err == nil {
		t.Fatal("expected error from Close with failing closer")
	}

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("expected ErrExportWriteFailed, got %v", err)
	}
}

// When Close is called after a prior streaming error, it must skip the
// buf.Flush() call (s.err != nil) and still return the existing error.
func TestNDJSONStreamer_CloseAfterError(t *testing.T) {
	t.Parallel()

	streamer := auditlog.NewNDJSONStreamer(
		&testhelpers.FailingWriter{},
		auditlog.WithBufferSize(1),
	)

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "err-before-close"},
	})

	if streamer.Err() == nil {
		t.Fatal("expected error before Close")
	}

	err := streamer.Close()
	if err == nil {
		t.Fatal("expected error from Close after prior error")
	}

	if !errors.Is(err, auditlog.ErrRenderFailed) {
		t.Errorf("expected ErrRenderFailed from Close, got %v", err)
	}
}

// Close must surface a deferred flush error even when no prior error was set
// (event fit in the 64 KB buffer, so OnEvent never saw the write failure).
func TestNDJSONStreamer_CloseFlushError(t *testing.T) {
	t.Parallel()

	streamer := auditlog.NewNDJSONStreamer(&testhelpers.FailingWriter{})

	streamer.OnEvent(auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "deferred-flush"},
	})

	// No error yet — event was buffered.
	if streamer.Err() != nil {
		t.Fatalf("unexpected error before Close: %v", streamer.Err())
	}

	err := streamer.Close()
	if err == nil {
		t.Fatal("expected error from Close when flush fails")
	}

	if !errors.Is(err, auditlog.ErrExportWriteFailed) {
		t.Errorf("expected ErrExportWriteFailed, got %v", err)
	}
}

// --- Helpers ---

// closeFailWriter is an io.ReadWriteCloser whose Write succeeds (delegating to
// the embedded bytes.Buffer) but Close always fails. Used to exercise the
// closer.Close() error path in NDJSONStreamer.Close.
type closeFailWriter struct {
	bytes.Buffer
}

func (*closeFailWriter) Close() error {
	return errors.New("close failed")
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

// --- Edge case tests ---

// WithBufferSize(0) and WithBufferSize(-1) should be ignored, keeping the
// default 64 KB buffer. The streamer should still function correctly.
func TestNDJSONStreamer_WithBufferSizeZeroOrNegative(t *testing.T) {
	t.Parallel()

	for _, size := range []int{0, -1, -100} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			streamer := auditlog.NewNDJSONStreamer(&buf, auditlog.WithBufferSize(size))

			for i := range 5 {
				streamer.OnEvent(auditlog.Event{
					Sequence:  i + 1,
					EventType: auditlog.EventTypeAttemptStart,
					Phase:     auditlog.PhaseBefore,
					StepRef:   auditlog.StepRef{Name: fmt.Sprintf("step-%d", i)},
				})
			}

			err := streamer.Flush()
			if err != nil {
				t.Fatalf("Flush: %v", err)
			}

			events, err := auditlog.ReadEvents(&buf)
			if err != nil {
				t.Fatalf("ReadEvents: %v", err)
			}

			if len(events) != 5 {
				t.Errorf("expected 5 events, got %d", len(events))
			}
		})
	}
}

// MaxEvents + streaming interaction: the streamer should see ALL events,
// including those dropped by Config.MaxEvents. The report should only contain
// the capped number.
func TestNDJSONStreamer_MaxEventsInteraction(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf)

	a, err := auditlog.New(auditlog.Config{
		Enabled:    true,
		MaxEvents:  3, // cap stored events at 3
		OnEvent:    streamer.OnEvent,
		WorkflowID: "maxevents-test",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	w := &flow.Workflow{}

	for i := range 5 {
		step := testhelpers.NewSucceed(fmt.Sprintf("step-%d", i))
		w.Add(flow.Step(step))
	}

	a.Attach(w)
	_ = w.Do(t.Context())
	a.Snapshot(w)

	err = streamer.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	streamedEvents, err := auditlog.ReadEvents(&buf)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	report := a.Report()

	if report.DroppedEventCount == 0 {
		t.Error("expected some dropped events due to MaxEvents cap")
	}

	if len(streamedEvents) <= report.EventCount {
		t.Errorf("streamer should see more events than stored: streamed=%d stored=%d",
			len(streamedEvents), report.EventCount)
	}

	t.Logf("streamed=%d events, stored=%d, dropped=%d",
		len(streamedEvents), report.EventCount, report.DroppedEventCount)
}

// Concurrent Close + OnEvent: verify no data race or panic when Close and
// OnEvent are called from different goroutines simultaneously.
func TestNDJSONStreamer_ConcurrentCloseAndOnEvent(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	streamer := auditlog.NewNDJSONStreamer(&buf)

	var wg sync.WaitGroup

	// Writer goroutine: continuously sends events.

	wg.Go(func() {
		for i := range 100 {
			streamer.OnEvent(auditlog.Event{
				Sequence:  i + 1,
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
				StepRef:   auditlog.StepRef{Name: "concurrent-step"},
			})
		}
	})

	// Closer goroutine: closes the streamer while events are still flowing.

	wg.Go(func() {
		_ = streamer.Close()
	})

	wg.Wait()

	// After both finish, a second Close should be safe (idempotent).
	err := streamer.Close()
	if err != nil {
		// Error from first close is fine, second close returns same error.
		_ = err
	}
}

// Property: streaming N events, reading them back, and replaying should
// produce equivalent event sequences. Run multiple iterations with varying
// event counts to exercise buffer boundary conditions.
func TestNDJSONStreamer_PropertyRoundTrip(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 2, 10, 50, 100, 200} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			streamer := auditlog.NewNDJSONStreamer(&buf)

			original := make([]auditlog.Event, 0, n)
			for i := range n {
				original = append(original, auditlog.Event{
					Sequence:  i + 1,
					EventType: auditlog.EventTypeAttemptStart,
					Phase:     auditlog.PhaseBefore,
					StepRef:   auditlog.StepRef{Name: fmt.Sprintf("step-%d", i)},
				})
			}

			for _, evt := range original {
				streamer.OnEvent(evt)
			}

			err := streamer.Flush()
			if err != nil {
				t.Fatalf("Flush: %v", err)
			}

			read, err := auditlog.ReadEvents(&buf)
			if err != nil {
				t.Fatalf("ReadEvents: %v", err)
			}

			if len(read) != n {
				t.Fatalf("expected %d events, got %d", n, len(read))
			}

			// Verify each event's sequence and name survived the round-trip.
			for i, evt := range read {
				if evt.Sequence != original[i].Sequence {
					t.Errorf("event %d: seq mismatch: got %d, want %d",
						i, evt.Sequence, original[i].Sequence)
				}

				if evt.Name != original[i].Name {
					t.Errorf("event %d: name mismatch: got %q, want %q",
						i, evt.Name, original[i].Name)
				}
			}
		})
	}
}

// --- Benchmarks ---

func benchmarkNDJSONStreamer(b *testing.B, n int) {
	b.Helper()

	events := make([]auditlog.Event, 0, n)

	for i := range n {
		events = append(events, auditlog.Event{
			Sequence:  i + 1,
			EventType: auditlog.EventTypeAttemptStart,
			Phase:     auditlog.PhaseBefore,
			StepRef:   auditlog.StepRef{Name: fmt.Sprintf("step-%d", i)},
		})
	}

	b.ResetTimer()

	for b.Loop() {
		s := auditlog.NewNDJSONStreamer(io.Discard)

		for _, evt := range events {
			s.OnEvent(evt)
		}

		_ = s.Flush()
	}
}

func BenchmarkNDJSONStreamer_100Events(b *testing.B) { benchmarkNDJSONStreamer(b, 100) }

func BenchmarkNDJSONStreamer_1000Events(b *testing.B) { benchmarkNDJSONStreamer(b, 1000) }

func BenchmarkNDJSONStreamer_10000Events(b *testing.B) {
	benchmarkNDJSONStreamer(b, 10000)
}
