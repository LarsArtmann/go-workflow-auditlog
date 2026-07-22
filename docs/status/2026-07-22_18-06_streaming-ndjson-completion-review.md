# Status Report: Streaming NDJSON Export — Completion & Self-Review

**Date**: 2026-07-22 18:06
**Session**: Continuation of streaming NDJSON export work (started in prior session, completed in this one)
**Branch**: master (9 commits ahead of origin)

---

## A) FULLY DONE

### Implementation (stream.go)

- **`NDJSONStreamer`** struct — thread-safe real-time NDJSON writer with `sync.Mutex`, `bufio.Writer`, `jsontext.Encoder`
- **`NewNDJSONStreamer(w, opts...)`** — constructor with 64KB default buffer
- **`CreateNDJSONStreamer(path, opts...)`** — file convenience constructor (direct writes, NOT atomic, for tailing)
- **`WithAutoFlush()`** — flush after every event
- **`WithBufferSize(n)`** — configurable buffer size (values <= 0 ignored, keeps default)
- **`OnEvent(evt Event)`** — thread-safe write, first-error-wins, drops events after error/close
- **`Flush() error`** — buffer flush, returns existing error if already failed
- **`Close() error`** — idempotent, flushes + closes underlying writer if it implements `io.Closer`
- **`Err() error`** — exposes first error encountered

### Code Quality

- **Extracted `encodeEvent`** shared helper in `export.go` — eliminated encoding duplication between `writeEventsNDJSON` and `NDJSONStreamer.OnEvent`. Both paths now produce identical NDJSON via one function.
- **Extracted `FailingWriter`/`ErrWriteFailed`** to `testhelpers` package — replaced 3 duplicate copies: `errorWriter` (stream_test.go), `failingWriter` (coverage_report_test.go), `failingWriter` + `errWriteFail` (viz/error_path_test.go)

### Tests (20 test functions in stream_test.go + example_test.go)

- `TestNDJSONStreamer_BasicRoundTrip` — write + read back + verify fields
- `TestNDJSONStreamer_EmptyFlush` — flush on empty streamer
- `TestNDJSONStreamer_ConcurrentSafety` — 16 goroutines × 50 events with `-race`
- `TestNDJSONStreamer_AutoFlush` — immediate write visibility
- `TestNDJSONStreamer_BufferedThenFlush` — buffered then flushed
- `TestNDJSONStreamer_ErrorHandling` — flush error surfaces
- `TestNDJSONStreamer_OnEventAfterError` — events dropped after error
- `TestNDJSONStreamer_EncodeError` — encode error path (buffer size 1 bypasses bufio)
- `TestNDJSONStreamer_FlushReturnsExistingError` — flush short-circuits on prior error
- `TestNDJSONStreamer_Close` — file close + data readable
- `TestNDJSONStreamer_DoubleClose` — idempotent
- `TestNDJSONStreamer_OnEventAfterClose` — events dropped after close
- `TestNDJSONStreamer_CloseError` — closer.Close() failure
- `TestNDJSONStreamer_CloseAfterError` — close skips flush when error exists
- `TestNDJSONStreamer_CloseFlushError` — deferred flush error surfaces on close
- `TestNDJSONStreamer_CreateFile` — file creation + round-trip
- `TestNDJSONStreamer_CreateFileError` — nonexistent path
- `TestNDJSONStreamer_NilErr` — nil error on fresh + successful streamer
- `TestNDJSONStreamer_WithBufferSize` — custom buffer size round-trip
- `TestNDJSONStreamer_WorkflowIntegration` — full Attach → Do → Snapshot → Flush with event comparison
- `TestNDJSONStreamer_AutoFlushWorkflowIntegration` — auto-flush in real workflow
- `ExampleNDJSONStreamer` — testable godoc example with `// Output:` verification
- `BenchmarkNDJSONStreamer_{100,1000,10000}Events` — throughput benchmarks to `io.Discard`

### Coverage

- **stream.go: 100%** on all 8 functions (WithAutoFlush, WithBufferSize, NewNDJSONStreamer, CreateNDJSONStreamer, OnEvent, Flush, Close, Err)
- **export.go `encodeEvent`: 100%**

### Documentation

- **doc.go** — added streaming mention to package doc
- **TODO_LIST.md** — checked the streaming checkbox (marked done)
- **FEATURES.md** — moved streaming from PLANNED to DONE section
- **AGENTS.md** — updated source file list (encodeEvent, WithBufferSize), documented MaxEvents + OnEvent interaction (streamer sees dropped events), updated test count (355), added FailingWriter to helpers table, updated benchmarks description

### Verification

- All 355 test functions pass with `-race` across both core and viz modules
- `golangci-lint` clean (0 issues) on both core (91 rules) and viz modules
- `go vet` clean on both modules

---

## B) PARTIALLY DONE

### Nothing is partially done — all items from the self-review action plan were completed.

---

## C) NOT STARTED

### Identified during this session but deferred:

1. **`writeEventsNDJSON` flush error path uncovered from core tests** (80% coverage) — The `buf.Flush()` error branch at `export.go:34-36` is tested from the viz test suite (`TestErrorPath_WriteNDJSON_FailingWriter_ErrExportWriteFailed` in `viz/error_path_test.go`) but NOT from any core test file. The core coverage report shows 0 hits on that branch. This is a pre-existing gap, not introduced by this session, but worth closing.

2. **No fuzz test for NDJSONStreamer** — The self-review recommended one (adversarial event payloads, concurrent writer failures). Not yet written.

3. **No property test for NDJSONStreamer** — Round-trip property (stream N events → read back → verify same N events with matching fields) would be valuable. Not yet written.

4. **README.md not updated** — The README is the end-user "sales page" and does not mention the streaming NDJSON feature. Users discovering the library through README won't know streaming exists.

5. **CHANGELOG.md not updated** — No changelog entry for the streaming feature.

6. **`WithFlushInterval` / `WithFlushEvery` time-based flush** — Mentioned in the self-review as a potential enhancement (flush every N milliseconds regardless of event count). Not implemented; the current `WithAutoFlush` (per-event) may be sufficient.

7. **MaxEvents + streaming integration test** — I documented that OnEvent fires for dropped events, but didn't add a test verifying the streamer sees more events than the recorder stores when `MaxEvents` is set.

---

## D) TOTALLY FUCKED UP

### Nothing. All changes are correct, tested, and lint-clean.

### Minor issues noticed but not harmful:

- **Commit history is noisy** — 9 commits ahead of origin with somewhat generic messages ("feat(stream): add streaming implementation with examples and test coverage", "test(stream): add comprehensive unit tests for stream functionality"). These were auto-generated. A squash merge or rebase would clean this up before pushing.
- **`writeEventsNDJSON` at 80% coverage from core** — Pre-existing gap (the flush error is tested from viz but not core). Not introduced by this session.
- **The `gci` formatter initially complained** about struct field alignment in `stream.go` — this was caught by lint and fixed during the session. The struct fields were re-aligned by the formatter.

---

## E) WHAT WE SHOULD IMPROVE

### Architecture & API Design

1. **No backpressure mechanism** — If the underlying writer blocks (slow network, full disk), the step goroutine calling `OnEvent` blocks too. A channel-based async writer with bounded queue would decouple step execution from I/O latency, but adds complexity. Current design is intentionally simple (mutex-serialized) and documented.

2. **Streaming doesn't capture Snapshot-enriched data** — Events streamed via `OnEvent` only contain callback-fired data. Steps settled by Conditions (Skipped/Canceled) bypass callbacks entirely. The streamed NDJSON is therefore incomplete for DAG reconstruction. Consumers who need the full picture must also call `Report()` after `Snapshot()`. This is a fundamental limitation of the callback-based approach and documented.

3. **`writeEventsNDJSON` coverage gap from core** — Should add a core test that exercises the flush error path directly, rather than relying on the viz test suite.

4. **`closeFailWriter` and `writeTracker` are local to `stream_test.go`** — Could be promoted to `testhelpers` if other tests need them, but each is currently used in only one place. Premature extraction.

### Testing

5. **No fuzz test** — Adversarial event payloads (malformed step names, nil fields, extreme sequence numbers) could reveal encoding edge cases.

6. **No property test** — A round-trip property test would verify `stream(events) → read back → same events` holds for arbitrary event sets.

7. **MaxEvents + streaming interaction untested** — The documentation says OnEvent fires for dropped events, but no test verifies this behavior.

8. **WithBufferSize(0) and negative values untested** — The code handles them (ignored, keep default) but there's no explicit test.

### Documentation

9. **README.md** — Should add a streaming section to the end-user guide with a usage example.

10. **CHANGELOG.md** — Should add an entry for the streaming feature.

### Code Style

11. **`NDJSONStreamer` struct field alignment** — The struct has 8 fields with varying alignment. `gci`/`gofumpt` handle this, but the field ordering (mutex first, then writer/buf/encoder, then error/config flags) follows Go convention correctly.

---

## F) Up to 50 Things to Get Done Next

#### High Priority (P0)

1. Add `writeEventsNDJSON` flush error test to core test suite (close the 80% → 100% gap)
2. Update `README.md` with streaming NDJSON section + usage example
3. Add `CHANGELOG.md` entry for streaming feature
4. Add MaxEvents + streaming integration test (verify streamer sees dropped events)
5. Squash/rebase the 9 noisy commits before pushing to origin

#### Medium Priority (P1)

6. Add fuzz test `FuzzNDJSONStreamer` — adversarial event payloads
7. Add property test — round-trip (stream → read → verify) for N random event sets
8. Add `WithBufferSize(0)` and `WithBufferSize(-1)` tests (verify default is kept)
9. Consider `WithFlushInterval(d time.Duration)` for time-based auto-flush
10. Add streaming example to `example/main.go` (the demo pipeline)
11. Verify `NDJSONStreamer` works with `io.Pipe` for streaming to HTTP responses
12. Add a `StreamingNDJSON` method on `Auditor` as a convenience wrapper? (design decision needed)
13. Consider whether streaming should be part of `Config` (e.g., `Config.NDJSONWriter io.Writer`) vs the current compose-it-yourself pattern
14. Add doc comment cross-references between `WriteNDJSON` (batch) and `NDJSONStreamer` (streaming) so users find both

#### Low Priority (P2)

15. Benchmark streaming with real file I/O (not just `io.Discard`) to measure actual disk throughput
16. Benchmark streaming with auto-flush vs buffered to quantify the latency/throughput tradeoff
17. Add `NDJSONStreamer` to the export formats table in `FEATURES.md` (currently in the text description, not the table)
18. Consider `CreateNDJSONStreamer` permissions (currently uses `os.Create` which uses 0666 before umask)
19. Add streaming integration test with sub-workflows (verify inner-step events are streamed)
20. Add streaming integration test with retry (verify attempt events are streamed)
21. Consider adding `NDJSONStreamer.EventsWritten() int64` counter for observability
22. Consider adding `NDJSONStreamer.BytesWritten() int64` counter
23. Add godoc cross-reference from `Config.OnEvent` to `NDJSONStreamer.OnEvent`
24. Review whether `encodeEvent` should be exported for external consumers (currently unexported)
25. Consider a `MultiWriter` that fans events to multiple `OnEvent` callbacks simultaneously
26. Add test for streaming with a very large event payload (multi-KB error message, long step names)
27. Add test for streaming with unicode/emoji step names (verify JSON escaping)
28. Consider whether `Close()` should return a wrapped error that distinguishes flush-failure from close-failure
29. Review thread-safety of `Err()` — it acquires the mutex, which is correct, but document the blocking behavior
30. Add integration test: stream → ReadEvents → ReplayEvents → compare with original Report
31. Consider adding streaming to the `viz/example` demo pipeline
32. Review whether `NDJSONStreamer` should implement `io.Closer` explicitly (it has `Close()` but doesn't implement the interface — it does implicitly)
33. Add test for concurrent `Close()` + `OnEvent()` (verify no race/panic when closing while events are being written)
34. Consider whether `Flush()` should be callable after `Close()` (currently it returns `s.err` which may be nil — but events are dropped, so the semantics are ambiguous)
35. Add test for `Flush()` after `Close()` (verify behavior is well-defined)
36. Consider adding `NDJSONStreamer.Reset(w io.Writer)` to reuse the streamer with a new writer
37. Add streaming to the STABILITY.md API stability matrix
38. Consider whether `WithBufferSize` should panic or silently ignore invalid values (currently silently ignores)
39. Add test for `CreateNDJSONStreamer` with existing file (verify it overwrites, matching `os.Create` behavior)
40. Add test for `CreateNDJSONStreamer` with read-only directory (verify error wrapping)
41. Consider streaming JSON report format (not just NDJSON events) — stream the full report as it's built
42. Add streaming support for the viz module (stream Mermaid/D2 diagrams incrementally as steps complete?)
43. Review whether `NDJSONStreamer` needs a `context.Context` parameter for cancellation
44. Consider adding `NDJSONStreamer.Done() <-chan struct{}` for signaling completion
45. Add test for streaming with `Config.Enabled = false` (verify OnEvent is never called)
46. Consider whether the streamer should handle `context.Canceled` errors gracefully
47. Add test for streaming with a workflow that has zero steps (verify no events, clean close)
48. Consider adding metrics hooks (e.g., `OnFlush func(bytesWritten int)`) for observability
49. Review the buffer size default (64KB) — is this optimal for typical audit event sizes (~200-500 bytes per event)?
50. Add a streaming-specific section to `docs/DOMAIN_LANGUAGE.md` (define "streaming", "auto-flush", "first-error-wins" terms)

---

## G) Questions (Cannot Figure Out Myself)

### 1. Should the noisy commit history be squashed before pushing?

The branch is 9 commits ahead of origin with somewhat generic, auto-generated commit messages. Options:

- **Squash to 1 clean commit** — simplest, cleanest history, but loses granular history
- **Interactive rebase into 2-3 logical commits** — preserves structure (implementation, tests, docs)
- **Push as-is** — fastest, but pollutes history

This is a user preference — I cannot decide this without your input.

### 2. Should streaming be promoted into `Config` or stay compose-it-yourself?

Current design: user creates `NDJSONStreamer`, passes `streamer.OnEvent` to `Config.OnEvent`. This is flexible but requires the user to know about the streamer type.

Alternative: add `Config.StreamingNDJSONWriter io.Writer` field that internally creates an `NDJSONStreamer`. Simpler API but adds lifecycle ownership complexity (who calls `Close()`?).

This is a product/API design decision that affects the public API surface.

### 3. Is the "streamer sees dropped events" behavior (MaxEvents + OnEvent) correct?

When `Config.MaxEvents` is set and the cap is reached, `appendEventLocked` drops the event from in-memory storage, but `onEvent` still fires afterward (it's called outside the lock, unconditionally). This means a streamer sees MORE events than `report.Events()` returns.

**Arguments for current behavior**: streaming is for external consumers who want every event regardless of in-memory caps.
**Arguments against**: it's surprising that streamed event count != report event count.

Should we change this (gate OnEvent behind the MaxEvents check) or keep it and just document it? This needs a product decision.
