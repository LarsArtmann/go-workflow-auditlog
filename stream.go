package auditlog

import (
	"bufio"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"os"
	"sync"
)

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

	writer    io.Writer
	buf       *bufio.Writer
	encoder   *jsontext.Encoder
	err       error
	autoFlush bool
	closed    bool
}

// NDJSONStreamerOption configures an [NDJSONStreamer].
type NDJSONStreamerOption func(*NDJSONStreamer)

// WithAutoFlush enables automatic flushing after every event write, reducing
// latency at the cost of throughput. Use this for real-time monitoring
// pipelines where consumers tail the output file.
func WithAutoFlush() NDJSONStreamerOption {
	return func(s *NDJSONStreamer) { s.autoFlush = true }
}

// NewNDJSONStreamer creates an [NDJSONStreamer] that writes NDJSON lines to w.
// The streamer uses a 64 KB internal buffer; call [NDJSONStreamer.Flush] or
// [NDJSONStreamer.Close] to guarantee all buffered data is written.
func NewNDJSONStreamer(w io.Writer, opts ...NDJSONStreamerOption) *NDJSONStreamer {
	buf := bufio.NewWriterSize(w, fileWriteBufferSize)

	s := &NDJSONStreamer{
		writer:  w,
		buf:     buf,
		encoder: jsontext.NewEncoder(buf),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// CreateNDJSONStreamer creates a file at path and returns an [NDJSONStreamer]
// writing to it. The caller should call [NDJSONStreamer.Close] when done,
// which flushes the buffer and closes the file.
//
// Unlike [Auditor.ExportNDJSON] (which uses atomic temp-file + rename), this
// writes directly to path so consumers can tail the file in real time.
func CreateNDJSONStreamer(path string, opts ...NDJSONStreamerOption) (*NDJSONStreamer, error) {
	file, err := os.Create(path)
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

	err := json.MarshalEncode(s.encoder, evt,
		jsontext.EscapeForHTML(true),
		jsontext.EscapeForJS(true),
	)
	if err != nil {
		s.err = fmt.Errorf("%w: encode event %d: %w", ErrRenderFailed, evt.Sequence, err)
		return
	}

	if s.autoFlush {
		if flushErr := s.buf.Flush(); flushErr != nil {
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

	if err := s.buf.Flush(); err != nil {
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
		if err := s.buf.Flush(); err != nil {
			s.err = fmt.Errorf("%w: flush ndjson stream on close: %w", ErrExportWriteFailed, err)
		}
	}

	if closer, ok := s.writer.(io.Closer); ok {
		if err := closer.Close(); err != nil && s.err == nil {
			s.err = fmt.Errorf("%w: close ndjson stream writer: %w", ErrExportWriteFailed, err)
		}
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
