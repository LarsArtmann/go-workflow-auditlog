# Status: Live Real-Time Dashboard Module — 2026-07-23

> **Session focus**: Built a new `live/` sub-module providing a real-time HTTP dashboard with SSE streaming, live DAG updates, and incremental rendering.

---

## a) FULLY DONE

### New `live/` sub-module (2,406 lines across 8 files)

| File             | Lines | Purpose                                                                                   | Status |
| ---------------- | ----- | ----------------------------------------------------------------------------------------- | ------ |
| `hub.go`         | 155   | SSE subscriber registry, fan-out OnEvent, SignalComplete, non-blocking broadcast          | DONE   |
| `server.go`      | 387   | HTTP server: SSE handler, /api/report, /api/health, dashboard serving, New() convenience  | DONE   |
| `dashboard.go`   | 176   | HTML template assembly reusing viz CSS + live CSS + JS + daghtml graph JS                 | DONE   |
| `dashboard.css`  | 198   | Live-specific CSS: pulsing badge, connection status, step animations, placeholders        | DONE   |
| `dashboard.js`   | 895   | SSE client + incremental rendering engine with requestAnimationFrame batching             | DONE   |
| `server_test.go` | 449   | 14 tests covering dashboard HTML, health, report, SSE snapshot/live/complete/fanout, hub  | DONE   |
| `demo/main.go`   | 146   | Multi-step pipeline demo with retry (fetch → validate → transform/enrich → save → notify) | DONE   |
| `doc.go`         | —     | Package documentation                                                                     | DONE   |

### Supporting changes

- **`viz/html.go`**: Exported `DashboardCSS()`, `DashboardJS()`, `BuildDAGHTML()` for reuse by live module
- **`go.work`**: Added `./live` to workspace
- **`.golangci.yml`**: Added `demo/` lint exclusions, `live`/`viz`/`testhelpers` to depguard allow list, `net/http.Server` to exhaustruct excludes
- **`AGENTS.md`**: Updated module list (2 → 3 modules), added live commands table, live source file inventory, SSE data flow section

### Verification (ALL GREEN)

| Module            | Build | Vet | Test -race | Lint     |
| ----------------- | ----- | --- | ---------- | -------- |
| Core (`auditlog`) | OK    | OK  | OK         | 0 issues |
| Viz               | OK    | OK  | OK         | 0 issues |
| Live              | OK    | OK  | OK         | 0 issues |

### Smoke test results (via demo)

- Health endpoint: `{"status":"ok","events":16,"complete":true}` ✅
- Dashboard HTML: serves correctly with CSP, LIVE badge, embedded CSS/JS ✅
- SSE snapshot: delivers report + events + metadata + DAG in first event ✅
- Demo workflow: 6 steps, 16 events, succeeded=true (including retry step with 3 attempts) ✅

---

## b) PARTIALLY DONE

### Dashboard JS rendering engine

- **Steps table**: Works but rebuilds entire table on every render tick. For large workflows (100+ steps), this causes flicker. Should diff-patch instead.
- **DAG Graph during execution**: Shows placeholder until `complete` event arrives. Live graph updates (adding nodes as steps start) are NOT implemented — the graph only renders once on `snapshot` or `complete`.
- **Timeline/Gantt**: Only renders from `report.steps` which requires `Snapshot(w)` to have been called. During live execution, timeline shows placeholder.
- **Waveform**: Renders from the full events array on each tick. Works but inefficient for large event counts.

### Test coverage gaps

- No test for concurrent Subscribe/Unsubscribe during OnEvent fan-out
- No test for the demo binary itself
- No fuzz test for SSE payload injection
- No test for heartbeat delivery
- No test for client reconnect behavior (JS-side)

---

## c) NOT STARTED

- **WebSocket transport**: SSE is one-way only. If we want client → server commands (e.g. "re-run step", "cancel workflow"), WebSocket would be needed.
- **Multi-run support**: The server handles a single workflow run. No support for multiple concurrent runs or run history.
- **Authentication**: No auth on any endpoint. Not suitable for production exposure without a reverse proxy.
- **TLS support**: HTTP only. No HTTPS configuration.
- **Compression**: No gzip/brotli on SSE or dashboard HTML.
- **NDJSON streaming integration**: The demo shows SSE only. No example of chaining NDJSONStreamer + live server simultaneously.
- **Graceful drain**: No mechanism to wait for all SSE clients to receive the `complete` event before shutting down.
- **Live DAG graph during execution**: The daghtml graph only renders from the final report. A "live DAG" that adds nodes/edges as steps start would need the workflow DAG structure before execution (which requires reading `w.Steps()` + `w.UpstreamOf()` before `Do()`).
- **Client-side replay**: No "playback" mode where you can scrub through events after completion.
- **Mobile responsiveness**: The dashboard CSS inherits from viz (which has a 768px breakpoint) but live-specific elements (stats grid, tabs) are not mobile-tested.

---

## d) TOTALLY FUCKED UP

- **Port 8080/9090 conflict**: Wasted significant time fighting a stale process on port 8080. The machine has system services bound to 8080 and 9090. Changed demo to port 18080. This was a time sink that should have been resolved in seconds with `lsof -i :PORT` but the toolchain banned `lsof`/`fuser`/`netstat`.
- **Initial JS syntax error**: Wrote `els waveform = els.waveform` (typo: `els` instead of `var`) in dashboard.js. Caught at build time but shouldn't have been written.
- **`json.NewEncoder` vs jsonv2**: First version used `json.NewEncoder(w)` which doesn't exist in the jsonv2 experiment. Had to switch to `jsontext.NewEncoder(w)` + `json.MarshalEncode`. Should have known this from reading existing code.
- **Data race on httpServer**: Initial `ListenAndServe`/`Shutdown` had unsynchronized access to `s.httpServer`. Caught by `-race` detector. Fixed with `sync.Mutex`.
- **Double-close on subscriber done channel**: `SignalComplete` closed `sub.done`, then `Unsubscribe` closed it again → panic. Fixed with `sync.Once`.

---

## e) WHAT WE SHOULD IMPROVE

### Architecture

1. **Live DAG during execution**: The biggest gap. Currently the graph only appears after completion. To show it live, we need to inject the DAG structure (from `w.Steps()` + `Traverse`) at `Attach()` time, before `Do()`. This would let the dashboard show the full graph skeleton immediately, with nodes lighting up as steps execute.
2. **Diff-based rendering**: The JS rebuilds entire DOM sections on each event. Should use keyed reconciliation (like React's virtual DOM) to patch only changed rows.
3. **Report polling fallback**: If SSE fails (corporate proxy, etc.), the dashboard should fall back to polling `/api/report` every 2s.
4. **Event coalescing**: During burst execution (many parallel steps), multiple events arrive in the same animation frame. The JS should batch them and render once.
5. **Backpressure**: If a client is slow, events are silently dropped (non-blocking send). The client should detect gaps in Sequence numbers and request a re-sync.

### Testing

6. **Integration test with real workflow**: No test runs an actual `flow.Workflow` through the live server. All tests inject events manually via `server.OnEvent()`. An end-to-end test would verify the full pipeline.
7. **Benchmark**: No benchmark for SSE fan-out performance with many clients.
8. **Browser test**: No headless browser test (Playwright/puppeteer) to verify the dashboard actually renders correctly in a browser.

### Developer Experience

9. **Auto-open browser**: The demo should optionally open the browser automatically (`open`/`xdg-open`).
10. **Configurable port via env**: `LIVE_PORT=8080 go run ./demo` would be friendlier than editing source.
11. **Verbose logging**: No structured logging in the server. Hard to debug connection issues.

### Polish

12. **Dark/light theme toggle**: The dashboard is dark-only. The CSS variables make this easy to add.
13. **Sound notifications**: Optional audio cue when a step fails.
14. **Progress bar**: Overall workflow progress bar based on completed/total steps.
15. **Export from live**: "Download JSON Report" button on the live dashboard.

---

## f) Up to 50 Things to Get Done Next

### High Priority (P0)

1. **Live DAG graph during execution** — inject DAG structure at Attach() time so nodes appear before execution starts
2. **End-to-end integration test** — run real `flow.Workflow` through live server, verify SSE delivery
3. **Report polling fallback** — if EventSource fails, poll `/api/report` every 2s
4. **Event sequence gap detection** — client detects missing Sequence numbers and requests re-sync
5. **Diff-based DOM rendering** — keyed reconciliation instead of full table rebuilds

### Medium Priority (P1)

6. **Multi-run support** — track multiple workflow runs, show run list, switch between them
7. **Run history persistence** — save completed runs to disk, allow replay
8. **Client-side event playback** — scrub through timeline after completion
9. **WebSocket transport** — for client→server commands (cancel, retry)
10. **Authentication** — bearer token or basic auth on endpoints
11. **TLS/HTTPS support** — built-in TLS configuration
12. **Compression** — gzip on dashboard HTML and SSE responses
13. **Graceful drain** — wait for clients to receive `complete` before shutdown
14. **Structured logging** — slog-based request logging
15. **NDJSON + live chaining example** — demo both streaming paths simultaneously
16. **Benchmark SSE fan-out** — measure throughput with 10/50/100 concurrent clients
17. **Headless browser test** — Playwright test verifying dashboard renders
18. **Progress bar** — overall workflow completion percentage
19. **Export from live UI** — download JSON/NDJSON/HTML from dashboard
20. **Configurable port via env var** — `LIVE_PORT`
21. **Auto-open browser** — optional `--open` flag on demo

### Lower Priority (P2)

22. **Dark/light theme toggle**
23. **Sound notifications on failure**
24. **Mobile responsive testing** — verify on 375px viewport
25. **Step detail panel** — click a step row to see full event history for that step
26. **Filter by step name in graph** — highlight matching nodes
27. **Critical path live indicator** — show critical path as it forms
28. **Peak concurrency live counter** — animate as it changes
29. **Wall clock live timer** — ticking stopwatch in the header
30. **Retry badge animations** — pulse when a retry happens
31. **Error grouping** — collapse repeated errors into a count
32. **Custom step metadata** — allow steps to attach arbitrary metadata
33. **Step duration predictions** — based on historical data
34. **Workflow comparison** — side-by-side live comparison of two runs
35. **Prometheus metrics endpoint** — `/metrics` for scraping
36. **OpenTelemetry integration** — correlate SSE events with traces
37. **Rate limiting** — per-client rate limit on SSE
38. **Connection ID display** — show client count in real-time
39. **Server-Sent Events spec compliance audit** — `Last-Event-ID` header support
40. **Reconnect with `Last-Event-ID`** — resume from last seen event on reconnect

### Cleanup / Tech Debt (P3)

41. **Extract shared JS utilities** — the live `dashboard.js` duplicates `esc()`, `humanizeDuration()` from viz `dashboard.js`. Should share via a common JS file.
42. **Golden file test for live dashboard HTML** — verify template structure doesn't regress
43. **CSS variable audit** — ensure live CSS doesn't override viz base values
44. **JSDoc comments on dashboard.js** — document the SSE protocol in the JS
45. **godoc examples for live.New and live.NewServer** — runnable examples
46. **Version constant alignment** — `live.SchemaVersion` is hardcoded "0.1.0"; should import from core
47. **Health endpoint struct export** — `healthResponse` is unexported but useful for testing
48. **Demo step types** — duplicate of testhelpers step types; could share
49. **go.work.sum sync** — verify workspace sums are consistent
50. **CI pipeline** — add live module to GitHub Actions workflow

---

## g) Questions

### 1. Should the live DAG graph show nodes before execution starts?

Currently the graph only renders after `Snapshot(w)` because dependencies are unknown until then. To show the graph during execution, I'd need to call `snapshotWorkflow(w)` at `Attach()` time (before `Do()`). However, `w.StateOf(step)` and `w.UpstreamOf(step)` may not return useful data before execution. **Do you want me to investigate whether go-workflow exposes the DAG structure before execution starts?**

### 2. Should we add WebSocket support or stick with SSE?

SSE is simpler and covers the one-way data flow (server → browser). WebSocket would enable client→server commands like "cancel workflow", "retry step", "jump to step". **Do you envision the dashboard ever needing to control the workflow, or is it observation-only?**

### 3. Should this be a separate versioned module or merged into viz?

Currently `live/` is a separate Go module (like `viz/`), depending on both `auditlog` and `viz`. An alternative is to merge it into `viz` as a `viz/live` sub-package, reducing module count. The tradeoff: `net/http` as a dependency in viz vs. keeping it isolated. **Do you prefer the current 3-module split, or should live move into viz?**
