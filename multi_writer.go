package auditlog

import "sync"

// MultiWriterCallback is the per-event callback signature accepted by
// [MultiWriter]. Returns a non-nil error to signal a per-consumer failure
// to the MultiWriter's OnEvent caller. The error is reported via the
// first-error-wins policy documented on [MultiWriter.OnEvent].
type MultiWriterCallback func(Event) error

// MultiWriter fans each event out to multiple OnEvent-style callbacks.
//
// Use it when a single workflow run needs to drive multiple consumers
// simultaneously — for example, streaming to an NDJSON file AND a live SSE
// hub AND an OpenTelemetry bridge — without composing them manually at the
// call site.
//
// All callbacks run on the goroutine that invokes OnEvent. If any callback
// blocks, it blocks the whole fan-out. Callbacks that need to do slow work
// (disk I/O, network calls) should hand the event off to their own
// goroutine before doing it.
//
// MultiWriter is safe for concurrent use by multiple goroutines (matching
// [NDJSONStreamer.OnEvent] concurrency guarantees): the internal mutex
// serializes the fan-out so callbacks never see a half-written event
// interleaving from another goroutine.
//
// Callbacks are fixed at construction time. There is no Add/Remove API —
// callers who need dynamic membership should swap the MultiWriter via a
// single atomic pointer (not provided here; out of scope for the common
// case of static composition).
type MultiWriter struct {
	mu        sync.Mutex
	callbacks []MultiWriterCallback
}

// NewMultiWriter returns a MultiWriter that invokes every fn for each event.
// At least one callback must be supplied; an empty list returns nil and the
// caller should treat that as a no-op sink.
func NewMultiWriter(fn ...MultiWriterCallback) *MultiWriter {
	if len(fn) == 0 {
		return nil
	}

	return &MultiWriter{callbacks: fn}
}

// OnEvent invokes every registered callback with evt, in registration order.
// Holds an internal mutex during fan-out so callbacks see no concurrent
// interleaving from other goroutines calling OnEvent.
//
// Returns the first non-nil error returned by any callback. Subsequent
// callbacks still run (no early termination) — matching the fail-open
// behavior of the underlying recorder, where one bad consumer must not
// silently drop events for the others.
func (m *MultiWriter) OnEvent(evt Event) error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error

	for _, fn := range m.callbacks {
		if fn == nil {
			continue
		}

		if err := fn(evt); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// CallbackCount returns the number of registered callbacks. Useful for
// debugging and tests.
func (m *MultiWriter) CallbackCount() int {
	if m == nil {
		return 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.callbacks)
}
