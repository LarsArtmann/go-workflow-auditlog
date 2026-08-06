# Status Report: go-sse Adoption Implementation

**Date:** 2026-08-06 20:14
**Session goal:** Execute the SUPERB go-sse adoption plan (`docs/planning/2026-08-06_19-45_SUPERB-go-sse-adoption-plan.md`)
**Status:** ~85% complete — all code implemented and tested, documentation mostly done, several gaps found

> **Resolution (2026-08-06):** All 3 questions in §g resolved or routed to TODO_LIST. Version drift (§b) fixed in FEATURES.md + AGENTS.md. Documentation gaps (§b) addressed in subsequent sessions.

---

## a) FULLY DONE ✅

### Phase A: Foundation (v0.4.0 upgrade + Stream adoption)

- **go-sse v0.3.0 → v0.4.0** bumped in `live/go.mod` — zero breaking changes, tests green
- **`sse.Stream` adopted in `handleSSE`** — replaced ~40 lines of manual SSE plumbing:
  - Manual header block (`Content-Type`, `Cache-Control`, `Connection`) → `sse.NewStream(w, r)`
  - Manual `flusher, ok := w.(http.Flusher)` check → Stream's internal flusher
  - Manual `sse.WriteEvent` + `flusher.Flush()` → `stream.Send(evt)`
  - Manual heartbeat ticker + `heartbeat.C` select case → `go stream.Heartbeat(r.Context(), interval)`
- **`sendSnapshot` and `sendComplete`** refactored to take `*sse.Stream` instead of `(http.ResponseWriter, http.Flusher)`
- **`X-Accel-Buffering: no`** preserved (set before `NewStream` — `SetHeaders` doesn't touch it)
- **All internal tests updated** for new signatures (`server_internal_test.go`)
- **Heartbeat test updated** — `TestServer_HandleSSE_Heartbeat` still passes unchanged (verifies `: heartbeat` comment frames from `Stream.Heartbeat` goroutine)
- **Write-failure test updated** — `TestServer_HandleSSE_HeartbeatWriteFailure` renamed to `TestServer_HandleSSE_WriteFailureAfterSnapshot` (heartbeat goroutine exits silently on write failure; test now covers the event `Send` failure path after snapshot flush)

### Phase B: Reconnection Replay (customer-facing feature)

- **`live/replay.go`** — new file with `eventRingBuffer` struct:
  - Bounded, thread-safe ring buffer implementing `sse.EventStore`
  - `add(sse.Event)` — appends with FIFO eviction when full
  - `EventsAfter(lastID sse.EventID)` — returns events with IDs strictly greater than lastID
  - `len()` — returns count for health endpoint
  - Default capacity: 1000 events
- **`live/hub.go`** — Hub updated:
  - `eventSeq atomic.Uint64` — monotonic event ID counter (no mutex needed)
  - `OnEvent` now assigns sequential SSE event IDs, stores in ring buffer, broadcasts `BroadcastEvent{ID, Data}`
  - `BroadcastEvent` struct exported (SSE `ID` field + `jsontext.Value` data) — replaces raw `jsontext.Value` channel type
  - `EventStore()` method — returns ring buffer as `sse.EventStore`
  - `BufferedEventCount()` method — returns ring buffer size for health endpoint
  - `NewHubWithReplay(capacity)` constructor — configurable buffer size
- **`live/server.go`** — replay wired into `handleSSE`:
  - Reads `Last-Event-ID` via `stream.LastEventID()` (browser sends automatically on reconnect)
  - Calls `sse.Replay(stream, srv.hub.EventStore(), lastID)` before snapshot if non-zero
  - `ReplayBufferSize` added to `Config` struct (default 1000)
  - `New()` convenience constructor passes `ReplayBufferSize` to `NewHubWithReplay`
- **`live/websocket.go`** — adapted to use `BroadcastEvent.Data` (WebSocket ignores the SSE ID)
- **`live/server_test.go`** — `TestHub_OnEventDelivery` updated for `BroadcastEvent.Data` field
- **`live/replay_test.go`** — 6 new tests:
  - `TestServer_SSE_ReconnectionReplay` — broadcast 3 events, reconnect with `Last-Event-ID: 1`, verify events 2+3 replayed
  - `TestServer_SSE_NoReplayOnInitialConnection` — connect without `Last-Event-ID`, verify no events replayed
  - `TestHub_EventStore_EventsAfterUnknownID` — lastID beyond all stored events → empty result
  - `TestHub_EventStore_ReplayMatchingEvents` — 5 events stored, request after ID "2" → events 3,4,5
  - `TestHub_RingBufferOverflow` — buffer capacity 3, broadcast 5 → oldest 2 evicted
  - `TestHub_EventStore_ConcurrentReplaySafety` — 4 broadcasters + 4 readers concurrently, race-clean

### Phase C: Graceful Shutdown Drain + Health Integration

- **`Hub.Drain(ctx)`** — waits for subscriber channel buffers to empty or context timeout (1ms poll interval, same pattern as go-sse's `fanOut.Shutdown`)
- **`Hub.IsDraining()`** — returns drain state for health endpoint
- **`Server.Shutdown(ctx)`** — now calls `hub.Drain(ctx)` before `httpServer.Shutdown(ctx)`
- **`healthResponse`** struct — new fields `Draining bool` and `EventBuffer int`
- **`live/lifecycle_test.go`** — 3 new tests:
  - `TestHub_Drain_DeliversBufferedEvents` — 10 events buffered, consumer goroutine drains, Drain completes
  - `TestHub_Drain_Timeout` — 5ms timeout with unconsumed events → Drain returns error
  - `TestServer_Health_ReportsDrainState` — health endpoint includes `draining` and `event_buffer_size`

### Phase D: Dashboard UX

- Reconnection indicators already existed in `dashboard.js` (`setConnStatus("reconnecting", "reconnecting...")` on `EventSource.onerror`)
- Browser's `EventSource` API automatically sends `Last-Event-ID` on reconnect — server-side replay handles it transparently
- `TestDashboardJS_UsesEventSource` updated with assertion for `"reconnecting"` status indicator

### Phase E: Documentation

- **CHANGELOG.md** — Added: reconnection replay, graceful drain, SSE event IDs, new Hub methods, Stream adoption, go-sse v0.4.0. Changed: handleSSE refactor, Hub broadcast type, Server.Shutdown drain, health endpoint fields
- **AGENTS.md** — Updated go-sse section (v0.4.0, Stream/Replay/EventStore adoption), updated live module source file listing (hub.go, replay.go, server.go descriptions), updated dependency versions
- **FEATURES.md** — Added reconnection replay feature, graceful drain feature, updated SSE heartbeat description, updated go-sse version references

### Phase F: Verification

- **Core tests:** `GOEXPERIMENT=jsonv2 go test -race -count=1 ./...` → PASS
- **Viz tests:** `cd viz && GOEXPERIMENT=jsonv2 go test -race -count=1 ./...` → PASS
- **Live tests:** `cd live && GOEXPERIMENT=jsonv2 go test -race -count=1 ./...` → PASS (77 tests, 0 failures)
- **Coverage:** 95.2% of statements (live module)
- **Lint:** 0 issues across all 3 modules
- **Vet:** clean across all 3 modules
- **No `replace` directives** in any go.mod

### Test counts

- Live module: 77 tests (was 68 before this session — 9 new tests added)
- Coverage per changed file: `hub.go` 95%+, `replay.go` 90%+, `server.go` 90%+, `websocket.go` 83%+

---

## b) PARTIALLY DONE ⚠️

### Documentation version drift (pre-existing, worsened by not fixing)

- **FEATURES.md** still references stale dependency versions that predate this session:
  - `go-output` at v0.31.1 → **actual: v0.35.0** in all go.mod files
  - `go-error-family` at v0.9.0 → **actual: v0.10.0**
  - `go-atomic-write` at v0.3.0 → **actual: v0.4.1**
  - I updated go-sse from v0.2.0 → v0.4.0 but did NOT fix the other three stale versions
- **AGENTS.md** has the same issue in the Gotchas section (line 231): references `go-output root v0.31.1` and `daghtml v0.31.1` — actual is v0.35.0. I updated the top-level dependency list (line 68) correctly but missed the deep references in the Gotchas bullets.

### AGENTS.md Concurrency Model section

- The Concurrency Model section still says "Hub broadcasts `jsontext.Value`" — should be updated to mention `BroadcastEvent{ID, Data}` and the ring buffer. I updated the file listing but not the concurrency model description.

### Planning doc not marked as executed

- `docs/planning/2026-08-06_19-45_SUPERB-go-sse-adoption-plan.md` still says "Status: Planning — awaiting approval before execution". Should be updated to "Status: Executed" with a reference to this status report.

---

## c) NOT STARTED ⏭️

### M15: Hub → Broadcaster refactor (DEFERRED — by design)

- The plan explicitly defers this. The Hub has ~60 lines of domain-specific code that Broadcaster doesn't provide. The actual eliminable duplication is ~15-30 lines. API shapes are incompatible. **Correctly skipped.**

### No benchmark for replay path

- The plan's testing patterns mention benchmarks for existing features. I did not add a benchmark for replay throughput (e.g., `BenchmarkReplay_1000Events`).

### No fuzz test for EventsAfter

- No fuzz target for adversarial EventID values in the ring buffer.

### No e2e test through real EventSource reconnection

- `TestServer_SSE_ReconnectionReplay` uses raw `http.Client` + manual `Last-Event-ID` header. A true e2e test would simulate: connect SSE → receive events → drop connection → reconnect (browser sends Last-Event-ID automatically) → verify missed events arrive. This requires a headless browser or SSE client library that mimics EventSource reconnection behavior. The current test validates the server-side replay logic correctly but doesn't test the full browser reconnection cycle.

### No integration test for Server.Shutdown → Drain → client receives buffered events

- `TestHub_Drain_DeliversBufferedEvents` tests Drain in isolation. `TestServer_GracefulShutdown` tests shutdown. But no test connects an SSE client, buffers events, calls `Server.Shutdown`, and verifies the client received the buffered events before the connection closed.

### No test for replay write-error path

- `handleSSE` has `if _, err := sse.Replay(stream, srv.hub.EventStore(), lastID); err != nil { return }` — no test exercises the error branch where `stream.Send` fails during replay.

---

## d) TOTALLY FUCKED UP 💥

### Nothing is totally fucked up.

- All code compiles, all tests pass, all lint is clean, all vet is clean.
- The auto-commit daemon committed two clean commits:
  - `14144a9 chore(deps): bump go-sse dependency from v0.3.0 to v0.4.0`
  - `f58562e feat(live): add SSE reconnection replay and graceful shutdown drain`
- The FEATURES.md fix (un-demo-merged text) is uncommitted in the working tree.

### Near-miss: FEATURES.md text merge

- When editing FEATURES.md, my `multiedit` for the graceful drain bullet accidentally merged with the "Demo pipeline" bullet text, producing: `...event_buffer_size state. at live/demo (fetch → validate...)`. I caught and fixed this, but the fix is in the uncommitted working tree, not in the auto-commit.

---

## e) WHAT WE SHOULD IMPROVE 🔧

### Code quality improvements

1. **Coverage gaps in error branches** — `Hub.OnEvent` at 85.7% (json.Marshal error branch untested), `EventsAfter` at 84.6% (ParseUint error on individual event IDs untested — the "unknown lastID" test covers the header parse, but not a corrupt event ID inside the buffer). These are edge cases but should be covered for a library claiming 95%+ coverage.
2. **`BroadcastEvent` is a public type** — it's exported because the external test package reads from `sub.Events()`. But its naming and documentation don't explain WHY it's public (it's an internal channel type that leaked into the public API via `Events()` method). Should either be unexported with an internal test, or properly documented as a public type.
3. **Ring buffer uses slice shift** — `rb.events = rb.events[1:]` on overflow is O(n) due to slice element shifting. For a 1000-element buffer at high event throughput, this could be a hot path. A proper ring buffer with head/tail indices would be O(1). Not a problem at current scale (workflow events are not high-frequency), but architecturally inelegant.
4. **Drain poll interval is hardcoded** — `drainPollInterval = time.Millisecond` is not configurable. For production use with many subscribers, a slightly longer interval might reduce CPU overhead during drain. Matches go-sse's pattern, so acceptable.
5. **`eventNameEvent` constant** — extracted to satisfy `goconst` linter (3 occurrences of `"event"`). The constant is package-private but could arguably be shared with the WebSocket handler which uses the same string. Currently both use it correctly.

### Testing improvements

6. **No WebSocket replay** — WebSocket clients that disconnect and reconnect get only the snapshot (Last-Event-ID is SSE-specific). This is by design, but there's no test asserting this limitation and no documentation warning WS consumers.
7. **No test for `Config.ReplayBufferSize`** — the config field exists and is wired through `New()` → `NewHubWithReplay()`, but there's no test that sets a custom buffer size via `live.Config` and verifies it takes effect (only `NewHubWithReplay(3)` is tested directly on the Hub).
8. **No test for `NewHubWithReplay(0)`** — the fallback-to-default path (capacity ≤ 0 → defaultReplayBufferSize) is not tested.

### Documentation improvements

9. **FEATURES.md stale versions** — go-output, go-error-family, go-atomic-write versions are wrong (predate this session). Should be fixed to match actual go.mod files.
10. **AGENTS.md stale versions** — deep references in Gotchas section still say go-output v0.31.1.
11. **AGENTS.md Concurrency Model** — doesn't mention the ring buffer or BroadcastEvent type.
12. **Planning doc status** — still says "awaiting approval before execution".

---

## f) Up to 50 Things We Should Get Done Next

#### High priority (fixes and gaps from this session)

1. Fix FEATURES.md stale dependency versions (go-output v0.35.0, go-error-family v0.10.0, go-atomic-write v0.4.1)
2. Fix AGENTS.md Gotchas section stale go-output version references (v0.31.1 → v0.35.0)
3. Update AGENTS.md Concurrency Model section to mention `BroadcastEvent`, ring buffer, and `atomic.Uint64` eventSeq
4. Mark planning doc as executed with link to this status report
5. Commit the uncommitted FEATURES.md fix (demo pipeline text un-merge)
6. Add test for `Config.ReplayBufferSize` wired through `live.New()` → verify custom capacity takes effect
7. Add test for `NewHubWithReplay(0)` fallback-to-default path
8. Add test for replay write-error path (stream.Send fails during sse.Replay)
9. Add integration test: Server.Shutdown → Drain → SSE client receives buffered events
10. Cover the `Hub.OnEvent` json.Marshal error branch (85.7% → 100%)

#### Medium priority (robustness and features)

11. Implement proper ring buffer with head/tail indices (O(1) instead of O(n) slice shift)
12. Add `BenchmarkReplay_1000Events` — measure replay throughput
13. Add `BenchmarkRingBuffer_Add` — measure event insertion performance
14. Add fuzz target for `EventsAfter` — adversarial EventID values
15. Document WebSocket replay limitation in FEATURES.md and AGENTS.md
16. Consider adding `sse.WriteRetry` to the initial SSE connection — tell the browser how long to wait before reconnecting
17. Add e2e test with real EventSource reconnection simulation (using an SSE client library)
18. Consider exposing `Hub.LastEventID()` for consumers who want to know the current sequence
19. Add `DrainTimeout` to `Config` (currently uses the Shutdown context deadline)
20. Consider using `Broadcaster.Shutdown` pattern from go-sse for the Hub (M15 deferred refactor — revisit if Hub grows)

#### Documentation and project health

21. Update FEATURES.md "Verified against the codebase on 2026-07-24" date
22. Update AGENTS.md test count (was 68 live tests, now 77)
23. Update AGENTS.md coverage number (was 95.5%, now 95.2%)
24. Add `replay.go` and `lifecycle_test.go` to AGENTS.md live module source files listing
25. Update CHANGELOG.md `[Unreleased]` → next version tag when ready
26. Consider whether `BroadcastEvent` should be unexported (move test to internal package)
27. Add godoc examples for `NewHubWithReplay` and `Hub.EventStore`
28. Document the `Cache-Control: no-transform` loss in FEATURES.md (currently only in AGENTS.md)

#### Testing infrastructure

29. Add race-condition stress test: many concurrent SSE clients reconnecting simultaneously
30. Add test: replay when workflow is already complete (replay + snapshot + complete in sequence)
31. Add test: ring buffer with capacity 1 (edge case)
32. Add test: EventsAfter with empty ring buffer (no events stored)
33. Add test: Drain with zero subscribers (should return immediately)
34. Add test: Drain called twice (idempotency)
35. Add test: health endpoint reports `event_buffer_size: 0` before any events

#### Broader project improvements (not session-specific)

36. Fix `gopls stdversion` warnings across all files (json/v2 requires go 1.27 flag — 29 warnings)
37. Consider upgrading `go 1.26.5` → `go 1.27` in all go.mod files to eliminate json/v2 warnings
38. Update FEATURES.md go-output version references everywhere (v0.31.1 → v0.35.0 appears 4+ times)
39. Consider adding `govulncheck` for the new go-sse v0.4.0 dependency
40. Review whether `sse.ContentType` should still be imported in server.go (now used only indirectly via NewStream)
41. Consider adding a `ReplayBufferSize` default constant to be shared between Config docs and ring buffer
42. Add a test that verifies the full SSE wire format includes `id:` lines (not just that replay works)
43. Consider telemetry/metrics for replay hit rate (how often clients reconnect with valid Last-Event-ID)
44. Review the `drainPollInterval` — 1ms might be too aggressive for production with many subscribers
45. Consider adding `Hub.Health()` method returning a structured snapshot (like `Broadcaster.Health`)
46. Add test for concurrent Drain + SignalComplete (what if drain is in progress when complete fires?)
47. Document the interaction between Drain and SignalComplete in hub.go
48. Consider whether `Server.Shutdown` should return the Drain error (currently silently ignored with `_ = srv.hub.Drain(ctx)`)
49. Review whether the ring buffer should be per-Hub or per-Server (currently per-Hub, which is correct)
50. Celebrate — the core adoption work is done and tested

---

## g) Questions I Cannot Answer Myself ❓

> **Resolved (2026-08-06):** Q1 deferred (acceptable as-is). Q2 DONE — version drift fixed in FEATURES.md + AGENTS.md this session. Q3 routed to TODO_LIST (cut v0.9.0).

1. **~~Should `Server.Shutdown` return the Drain error or silently ignore it?~~** Currently I do `_ = srv.hub.Drain(ctx)` — if the drain times out, the error is silently swallowed and shutdown proceeds to close HTTP anyway. The alternative is to return the drain error and let the caller decide (retry or force-close). go-sse's `Broadcaster.Shutdown` returns the error. Which behavior do you want for `live.Server`?

2. **~~Should I fix the pre-existing version drift in FEATURES.md and AGENTS.md now, or leave it for a separate docs-cleanup pass?~~** **DONE.** All version references updated to match go.mod files (go-output v0.35.0, go-error-family v0.10.0, go-atomic-write v0.4.1, go-branded-id v0.5.1).

3. **~~Do you want a v0.9.0 release cut with these changes, or should they accumulate with other pending work first?~~** Routed to TODO_LIST — cut v0.9.0 coordinated three-module release when ready.
