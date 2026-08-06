package auditlog_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestMultiWriter_FansOutToAllCallbacks(t *testing.T) {
	t.Parallel()

	var (
		wg sync.WaitGroup
		mu sync.Mutex

		got []auditlog.Event
	)

	callback := func(e auditlog.Event) error {
		mu.Lock()

		got = append(got, e)

		mu.Unlock()

		wg.Done()

		return nil
	}

	mw := auditlog.NewMultiWriter(callback, callback, callback)

	evt := auditlog.Event{
		Sequence:  1,
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
		StepRef:   auditlog.StepRef{Name: "s1"},
	}

	wg.Add(3)

	if err := mw.OnEvent(evt); err != nil {
		t.Fatalf("OnEvent: %v", err)
	}

	wg.Wait()

	if len(got) != 3 {
		t.Errorf("expected 3 callback invocations, got %d", len(got))
	}

	for i, e := range got {
		if e != evt {
			t.Errorf("invocation %d: got %+v, want %+v", i, e, evt)
		}
	}
}

func TestMultiWriter_PreservesRegistrationOrder(t *testing.T) {
	t.Parallel()

	var order []int

	cb := func(n int) auditlog.MultiWriterCallback {
		return func(auditlog.Event) error {
			order = append(order, n)

			return nil
		}
	}

	mw := auditlog.NewMultiWriter(cb(1), cb(2), cb(3))

	if err := mw.OnEvent(auditlog.Event{}); err != nil {
		t.Fatalf("OnEvent: %v", err)
	}

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("callback order violated: %v", order)
	}
}

func TestMultiWriter_ReturnsFirstError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("callback-2 failure")

	var thirdCalled atomic.Bool

	mw := auditlog.NewMultiWriter(
		func(auditlog.Event) error {
			return nil
		},
		func(auditlog.Event) error {
			return sentinel
		},
		func(auditlog.Event) error {
			thirdCalled.Store(true)

			return nil
		},
	)

	err := mw.OnEvent(auditlog.Event{})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}

	if !thirdCalled.Load() {
		t.Error("third callback must run even when an earlier callback errors (fail-open)")
	}
}

func TestMultiWriter_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	var counter atomic.Int64

	cb := func() auditlog.MultiWriterCallback {
		return func(auditlog.Event) error {
			counter.Add(1)

			return nil
		}
	}

	mw := auditlog.NewMultiWriter(cb(), cb(), cb())

	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)

		go func(seq int) {
			defer wg.Done()

			_ = mw.OnEvent(auditlog.Event{Sequence: seq})
		}(i)
	}

	wg.Wait()

	// 3 callbacks × 100 events = 300 invocations.
	if got := counter.Load(); got != 300 {
		t.Errorf("expected 300 total invocations, got %d", got)
	}
}

func TestMultiWriter_NoCallbacks(t *testing.T) {
	t.Parallel()

	if mw := auditlog.NewMultiWriter(); mw != nil {
		t.Errorf("NewMultiWriter() should return nil for empty input, got %+v", mw)
	}
}

func TestMultiWriter_NilReceiver(t *testing.T) {
	t.Parallel()

	var mw *auditlog.MultiWriter

	if err := mw.OnEvent(auditlog.Event{}); err != nil {
		t.Errorf("nil MultiWriter.OnEvent should return nil, got %v", err)
	}

	if got := mw.CallbackCount(); got != 0 {
		t.Errorf("nil MultiWriter.CallbackCount should return 0, got %d", got)
	}
}

func TestMultiWriter_SkipsNilCallback(t *testing.T) {
	t.Parallel()

	called := false

	mw := auditlog.NewMultiWriter(
		nil,
		func(auditlog.Event) error {
			called = true

			return nil
		},
		nil,
	)

	if err := mw.OnEvent(auditlog.Event{}); err != nil {
		t.Fatalf("OnEvent: %v", err)
	}

	if !called {
		t.Error("non-nil callback should run between nil callbacks")
	}
}

func TestMultiWriter_CallbackCount(t *testing.T) {
	t.Parallel()

	mw := auditlog.NewMultiWriter(
		func(auditlog.Event) error {
			return nil
		},
		func(auditlog.Event) error {
			return nil
		},
	)

	if got := mw.CallbackCount(); got != 2 {
		t.Errorf("CallbackCount = %d, want 2", got)
	}
}
