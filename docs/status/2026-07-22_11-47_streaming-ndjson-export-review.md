# Status Report: Streaming NDJSON Export Option

**Date:** 2026-07-22 11:47  
**Session Scope:** Implement streaming NDJSON export (`NDJSONStreamer`) in core module  
**Status:** FUNCTIONAL BUT INCOMPLETE — ships working code, misses tests/docs/features

---

## a) FULLY DONE

| Item | Details |
|------|---------|
| `stream.go` — `NDJSONStreamer` type | Thread-safe real-time NDJSON writer via `Config.OnEvent`. Mutex-serialized writes, 64KB buffer, `WithAutoFlush()` option, `CreateNDJSONStreamer(path)` file constructor, `Flush()`, `Close()` (idempotent, closes underlying writer), `Err()`, first-error-wins error handling. |
| `stream_test.go` — 16 tests | Basic round-trip, empty flush, concurrent safety (16x50 events with `-race`), auto-flush immediacy, buffered-then-flush, error handling (`errors.Is(ErrExportWriteFailed)`), post-error event suppression, close (flush+close+read), double-close idempotency, post-close drops, file creation, workflow integration (parallel succeed+fail), nonexistent-path error, nil-err, auto-flush workflow integration, full lifecycle. |
| Sentinel error reuse | Correctly reuses existing `ErrRenderFailed` (encode) and `ErrExportWriteFailed` (flush/close/write) — no new sentinels created. Already registered with go-error-family via `classify.go`. |
| Lint clean | 0 issues from golangci-lint (91 linters) on both core and viz modules. |
| All existing tests pass | `-race` clean across both modules. |
| `AGENTS.md` updated | Source file list, data flow diagram, gotchas section, test count (346). |

---

## b) PARTIALLY DONE

| Item | What exists | What's missing |
|------|-------------|----------------|
| Test coverage on `stream.go` | 5 of 7 functions at 100% | `OnEvent` 83.3%, `Flush` 88.9%, `Close` 87.5% — uncovered branches: the `autoFlush` flush-error path in `OnEvent`, the `s.err != nil` early-return in `Flush`, the `closer.Close()` error path in `Close` |
| AGENTS.md documentation | Source list, gotchas, data flow | Test count updated but no mention in architecture description of streaming as a first-class export path |
| API surface | `NewNDJSONStreamer`, `CreateNDJSONStreamer`, `WithAutoFlush`, `OnEvent`, `Flush`, `Close`, `Err` | No `WriteJSON` streaming equivalent; no buffer-size option |

---

## c) NOT STARTED

| Item | Why it matters |
|------|----------------|
| **`FEATURES.md` update** | Line 127 lists "Streaming NDJSON export" under "WORTH CONSIDERING" — should move to DONE/PARTIALLY DONE |
| **`TODO_LIST.md` update** | Line 15 has `- [ ] Streaming NDJSON export option` — checkbox not checked |
| **`doc.go` update** | Package doc says "export to JSON and NDJSON" but doesn't mention streaming |
| **Benchmark** (`BenchmarkNDJSONStreamer`) | Project benchmarks everything (Invocation, Attach, BuildReport, OnEventCallback, MermaidExport, renderHTML). No streaming throughput benchmark exists. |
| **Fuzz test** | Project has `FuzzReadEvents` for the NDJSON reader and fuzz tests for diagrams/HTML. No fuzz test for streaming encode/write paths. |
| **Property test** (streamed == batch) | Project has property tests for Diff algebra. A property test verifying "streaming output for N events == batch `WriteNDJSON` output" would catch encoding divergence. |
| **Godoc `Example` function** | I wrote `ExampleNDJSONStreamer` but **deleted it** during lint fixes instead of fixing it (needed `// Output:` comment). The project lists 11 examples as a test category. This was lazy. |
| **`testhelpers` additions** | Created local `errorWriter` and `writeTracker` helpers instead of sharing. `errorWriter` is a near-duplicate of `failingWriter` in `coverage_report_test.go`. |
| **README update** | If README lists features, streaming is not there. |

---

## d) TOTALLY FUCKED UP

### 1. The `ExampleNDJSONStreamer` function — DELETED INSTEAD OF FIXED

I wrote a godoc example, then when `testableexamples` linter complained "missing output", I **removed the entire function** instead of adding `// Output:` or restructuring it to be testable. The project values examples (11 exist). This was the lazy path. The example also passed `nil` as context to `w.Do()` which `staticcheck SA1012` flagged — another reason I should have fixed it properly.

**Fix:** Rewrite as a testable example with `// Output:` comment or convert to an `Example` that uses `os.Stdout` with deterministic output.

### 2. Duplicate failing-writer test helper — SPLIT BRAIN

I created `errorWriter` in `stream_test.go` while `failingWriter` already exists in `coverage_report_test.go`. Both do the exact same thing (`Write` returns error). I renamed mine to avoid a compile conflict instead of extracting a shared helper. Two types, same behavior, different names — classic split brain.

**Fix:** Extract `FailingWriter` to `testhelpers` package, use everywhere.

### 3. `TestNDJSONStreamer_FullLifecycleExample` is a duplicate of `TestNDJSONStreamer_WorkflowIntegration`

Both tests attach a streamer to an auditor, run a workflow, flush, and verify events. The only difference is one uses `testhelpers.RunWorkflow` and the other manually calls `Attach`/`Do`/`Snapshot`. This is redundant test bloat.

---

## e) WHAT WE SHOULD IMPROVE

### Architecture & Design

1. **No configurable buffer size.** `NDJSONStreamer` hardcodes `fileWriteBufferSize` (64KB). Users streaming to slow network sinks may want larger buffers; users needing near-zero latency may want smaller. Should be `WithBufferSize(n int)` option.

2. **Encoding logic duplicated.** Both `writeEventsNDJSON` (export.go) and `NDJSONStreamer.OnEvent` (stream.go) manually call `json.MarshalEncode` with identical options (`EscapeForHTML(true)`, `EscapeForJS(true)`). Should extract a shared `encodeEvent(encoder, evt) error` helper to guarantee they never diverge.

3. **MaxEvents + OnEvent interaction is undocumented.** When `Config.MaxEvents` is set and the cap is reached, `appendEventLocked` drops the event from internal storage, but `onEvent` callback **still fires** (recorder.go:129-138). This means a streamer receives events that the recorder dropped. This is arguably correct (MaxEvents protects memory, streaming is a separate sink), but it's a semantic surprise that MUST be documented.

4. **No periodic auto-flush.** `WithAutoFlush` flushes after every single event. For high-throughput workflows, a `WithFlushInterval(d time.Duration)` or `WithFlushEvery(n int)` option would batch-flush on a timer/counter, balancing latency and throughput.

5. **`CreateNDJSONStreamer` name is inconsistent.** Other constructors in the package use `New*` prefix (`NewNDJSONStreamer`, `NewRecorder`, `New`). `Create` implies file creation but the naming pattern is jarring. Consider `NewFileNDJSONStreamer` or document why it diverges.

6. **No `io.WriteCloser` interface conformance.** `NDJSONStreamer` has `Close() error` but doesn't implement `Write([]byte) (int, error)`, so it can't be used where `io.WriteCloser` is expected. Not necessarily wrong (it's an event consumer, not a byte writer), but worth noting.

### Testing

7. **Uncovered branches in stream.go.** `OnEvent` autoFlush error path, `Flush` error-already-set early return, `Close` closer-error path. Need targeted tests.

8. **No byte-for-byte equivalence test.** Should verify that for the same events, streaming output == `WriteNDJSON` output. Currently only tested semantically (read back and compare fields).

9. **No large-scale test.** Concurrent test uses 800 events (16x50). No test with thousands of events to verify buffer refill behavior.

10. **No test for MaxEvents + streaming interaction.** See point 3 above.

11. **No fuzz test for encode failure paths.** A malformed Event (e.g., with invalid time.Time) could cause `MarshalEncode` to fail — should fuzz this.

### Documentation

12. **`FEATURES.md` still says "WORTH CONSIDERING".** The feature is implemented. Update status.

13. **`TODO_LIST.md` checkbox unchecked.** Line 15 still `- [ ]`.

14. **`doc.go` doesn't mention streaming.** Package doc says "export to JSON and NDJSON" — should say "with streaming NDJSON support."

---

## f) Up to 50 Things to Get Done Next

### Critical (broken or embarrassing)

1. ✅ Fix `FEATURES.md` — move streaming from "WORTH CONSIDERING" to "DONE"
2. ✅ Check `TODO_LIST.md` box for streaming NDJSON export
3. ✅ Fix `doc.go` — mention streaming in package doc
4. ✅ Extract shared `FailingWriter` to `testhelpers`, deduplicate `errorWriter`/`failingWriter`
5. ✅ Delete or merge `TestNDJSONStreamer_FullLifecycleExample` with `TestNDJSONStreamer_WorkflowIntegration`
6. ✅ Document MaxEvents + OnEvent interaction in AGENTS.md gotchas

### High-value improvements

7. Extract `encodeEvent(encoder, evt) error` shared helper to eliminate encoding duplication
8. Add `WithBufferSize(n int)` option to `NDJSONStreamer`
9. Fix uncovered branches: test autoFlush error path, Flush error-set early return, Close closer-error
10. Add byte-for-byte equivalence test (streaming output == WriteNDJSON output)
11. Re-add `ExampleNDJSONStreamer` as a proper testable example with `// Output:`
12. Add `BenchmarkNDJSONStreamer` — throughput with 100/1000/10000 events
13. Add `FuzzNDJSONStreamer` — encode failure paths with malformed events
14. Add property test: streamed events set == batch events set for random workflows

### Medium-value features

15. Add `WithFlushInterval(d time.Duration)` for periodic auto-flush
16. Add `WithFlushEvery(n int)` for counter-based batch flush
17. Consider streaming JSON report option (not just NDJSON events)
18. Add `NDJSONStreamer` to README feature list (if applicable)
19. Add streaming example to `viz/example/` demo pipeline
20. Test with thousands of events to verify buffer refill behavior
21. Test MaxEvents + streaming interaction explicitly

### Lower priority

22. Consider `NDJSONStreamer` implementing a standard interface
23. Add streaming integration test with retry/timeout steps
24. Add streaming integration test with sub-workflows
25. Add streaming test with disabled auditor (should produce no output)
26. Verify standalone build (`GOWORK=off`) still works with new file
27. Consider `WithLineSeparator(sep string)` option
28. Consider metrics/callback for successful writes (monitoring)
29. Add streaming to the godoc package example in `doc.go`
30. Consider whether `CreateNDJSONStreamer` should use `WriteToFile` for atomic writes (trade-off: atomic vs tailable)
31. Update AGENTS.md "Testing Patterns" section to mention streaming tests
32. Consider `NDJSONStreamer.Reset()` to reuse the streamer for multiple runs
33. Add test for `NDJSONStreamer` with `os.Pipe()` (real OS pipe, not just `bytes.Buffer`)
34. Consider backpressure: what happens if the writer blocks? (Currently blocks the step goroutine)
35. Document that streaming does NOT capture Snapshot-enriched data (DAG structure, skipped/canceled status) — only callback-fired events
36. Consider a `MultiWriter` that streams to multiple sinks simultaneously
37. Add test verifying RunID is present in every streamed event
38. Add test verifying Sequence is monotonically increasing in streamed output for sequential steps
39. Consider whether `Close()` should also be called by `Auditor` if it owns the streamer
40. Add streaming-specific error path tests to viz module error-path test suite
41. Consider `Config.NDJSONWriter io.Writer` shorthand (auto-creates streamer internally)
42. Review whether streaming should be mentioned in `docs/DOMAIN_LANGUAGE.md`
43. Add changelog entry if a CHANGELOG.md exists
44. Consider whether streaming interacts correctly with `DroppedEventCount()`
45. Add test for streaming with `Config.InitialEventCapacity` set
46. Consider doc comment cross-references between `WriteNDJSON` and `NDJSONStreamer`
47. Add `// See also:` comments linking `ReadEvents` ↔ `NDJSONStreamer` ↔ `WriteNDJSON`
48. Consider whether `viz` module should offer a streaming visualization option
49. Review naming: `NDJSONStreamer` vs `Streamer` vs `EventStream` — is the name clear enough?
50. Consider integration test: stream to file, read back, replay, generate diagram — full pipeline

---

## g) Questions I CANNOT Figure Out Myself

### 1. Should the streamer be auto-wired via `Config.NDJSONWriter io.Writer`?

Currently streaming is compose-it-yourself: user creates `NDJSONStreamer`, passes `streamer.OnEvent` to `Config.OnEvent`. Should `Config` gain an `NDJSONWriter io.Writer` field that auto-creates and manages the streamer internally? This changes the API surface and lifecycle ownership model.

**My recommendation:** Keep compose-it-yourself. It's more flexible and follows the existing `OnEvent` pattern. But this is a product decision about how much magic the Config should have.

### 2. Is the MaxEvents + OnEvent interaction correct?

When `Config.MaxEvents` is reached, dropped events are removed from the recorder's internal slice but `OnEvent` still fires for them. This means:
- `report.WriteNDJSON()` writes N events (cap reached)
- `NDJSONStreamer` output has N + dropped events

Is this the intended behavior? Two valid interpretations:
- **Correct:** MaxEvents protects in-memory storage; streaming is a separate sink that should see everything
- **Bug:** MaxEvents should cap the entire observation; streaming should also drop

**My recommendation:** The current behavior is correct (MaxEvents = memory protection), but it needs explicit documentation and a test. However, if you consider it a bug, the fix is to gate `onEvent` behind the same `maxEvents` check.

### 3. Should streaming also support JSON report format (not just NDJSON events)?

NDJSON streams individual events in real time. A JSON report is a single document assembled post-execution. Streaming a JSON report is fundamentally different (you'd need to write a partial document and close it on Flush/Close). Should we build this, or is NDJSON event streaming sufficient?

**My recommendation:** NDJSON is sufficient for real-time use cases. JSON report is inherently a batch format. But if there's a monitoring use case for "partial report as steps complete," it's worth building.
