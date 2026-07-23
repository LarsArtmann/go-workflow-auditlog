package auditlog

import (
	"bufio"
	"encoding/json/jsontext"
	"fmt"
	"io"
	"os"
	"sync"
)

// ndjsonBufferSize is the default buffer size for NDJSON streaming writes.
const ndjsonBufferSize = 65536

// NDJSONStreamer writes events as newline-delimited JSON to an [io.Writer]
// in real time, as each event is captured during workflow execution.
//
// It is safe for concurrent use — the [Config.OnEvent] callback fires from
// multiple step goroutines, and the streamer serialises writes with an
// internal mutex so lines never interleave.
//
// Events are written in the order they arrive at the callback, which may differ
// from Sequence order when steps run concurrently. Consumers that need strict
// ordering should sort by [Event.Sequence] after reading the output back with
// [ReadEvents].
//
// # Usage
//
//	file, err := os.Create("audit.ndjson")
//	if err != nil { ... }
//	defer file.Close()
//
//	streamer := auditlog.NewNDJSONStreamer(file)
//	auditor, err := auditlog.New(auditlog.Config{
//		Enabled: true,
//		OnEvent: streamer.OnEvent,
//	})
//	// ... attach, run workflow, snapshot ...
//	if err := streamer.Flush(); err != nil { ... }
//
// For low-latency monitoring pipelines that must see each event immediately,
// pass [WithAutoFlush]:
//
//	streamer := auditlog.NewNDJSONStreamer(file, auditlog.WithAutoFlush())
type NDJSONStreamer struct {
	mu sync.Mutex

	writer     io.Writer
	buf        *bufio.Writer
	encoder    *jsontext.Encoder
	err        error
	bufferSize int
	autoFlush  bool
	closed     bool
}

// NDJSONStreamerOption configures an [NDJSONStreamer].
type NDJSONStreamerOption func(*NDJSONStreamer)

// WithAutoFlush enables automatic flushing after every event write, reducing
// latency at the cost of throughput. Use this for real-time monitoring
// pipelines where consumers tail the output file.
func WithAutoFlush() NDJSONStreamerOption {
	return func(s *NDJSONStreamer) { s.autoFlush = true }
}

// WithBufferSize sets the internal buffer size in bytes. The default is 64 KB
// (matching the atomic-file export path). Use a larger buffer for high-throughput
// bursty writes, or a smaller buffer for lower-latency streaming. Values <= 0
// are ignored and keep the default.
func WithBufferSize(size int) NDJSONStreamerOption {
	return func(s *NDJSONStreamer) {
		if size > 0 {
			s.bufferSize = size
		}
	}
}

// NewNDJSONStreamer creates an [NDJSONStreamer] that writes NDJSON lines to w.
// The streamer uses a 64 KB internal buffer by default; call
// [NDJSONStreamer.Flush] or [NDJSONStreamer.Close] to guarantee all buffered
// data is written.
func NewNDJSONStreamer(w io.Writer, opts ...NDJSONStreamerOption) *NDJSONStreamer {
	s := &NDJSONStreamer{
		writer:     w,
		bufferSize: ndjsonBufferSize,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.buf = bufio.NewWriterSize(w, s.bufferSize)
	s.encoder = jsontext.NewEncoder(s.buf)

	return s
}

// CreateNDJSONStreamer creates a file at path and returns an [NDJSONStreamer]
// writing to it. The caller should call [NDJSONStreamer.Close] when done,
// which flushes the buffer and closes the file.
//
// Unlike [Auditor.ExportNDJSON] (which uses atomic temp-file + rename), this
// writes directly to path so consumers can tail the file in real time.
func CreateNDJSONStreamer(path string, opts ...NDJSONStreamerOption) (*NDJSONStreamer, error) {
	file, err := os.Create(path) //nolint:gosec // path is user-provided by design
	if err != nil {
		return nil, fmt.Errorf("%w: create %q: %w", ErrExportWriteFailed, path, err)
	}

	return NewNDJSONStreamer(file, opts...), nil
}

// OnEvent writes the event as a single line of NDJSON.
// It is safe for concurrent use.
//
// If a previous write failed (see [NDJSONStreamer.Err]), subsequent events
// are silently dropped to avoid cascading errors.
func (s *NDJSONStreamer) OnEvent(evt Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil || s.closed {
		return
	}

	err := encodeEvent(s.encoder, evt)
	if err != nil {
		s.err = err

		return
	}

	if s.autoFlush {
		flushErr := s.buf.Flush()
		if flushErr != nil {
			s.err = fmt.Errorf("%w: flush ndjson stream: %w", ErrExportWriteFailed, flushErr)
		}
	}
}

// Flush writes any buffered data to the underlying [io.Writer].
// Returns the first error encountered during streaming, if any.
func (s *NDJSONStreamer) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}

	err := s.buf.Flush()
	if err != nil {
		s.err = fmt.Errorf("%w: flush ndjson stream: %w", ErrExportWriteFailed, err)

		return s.err
	}

	return nil
}

// Close flushes the buffer and, if the underlying writer implements
// [io.Closer], closes it. After Close, further calls to OnEvent are silently
// dropped. Close is idempotent — calling it multiple times returns the same
// error (or nil).
func (s *NDJSONStreamer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return s.err
	}

	s.closed = true

	if s.err == nil {
		err := s.buf.Flush()
		if err != nil {
			s.err = fmt.Errorf("%w: flush ndjson stream on close: %w", ErrExportWriteFailed, err)
		}
	}

	closer, ok := s.writer.(io.Closer)
	if !ok {
		return s.err
	}

	err := closer.Close()
	if err != nil && s.err == nil {
		s.err = fmt.Errorf("%w: close ndjson stream writer: %w", ErrExportWriteFailed, err)
	}

	return s.err
}

// Err returns the first error encountered during streaming, or nil if all
// writes succeeded.
func (s *NDJSONStreamer) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.err
}
