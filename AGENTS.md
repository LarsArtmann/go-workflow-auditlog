# AGENTS.md — go-workflow-auditlog

Go library for [Azure/go-workflow](https://github.com/Azure/go-workflow) that records every step execution event (attempts, retries, durations, errors, dependencies, final statuses) with timestamps and export to JSON / NDJSON.

**Modules**: `github.com/larsartmann/go-workflow-auditlog` (core) · `github.com/larsartmann/go-workflow-auditlog/viz` (visualization) · `github.com/larsartmann/go-workflow-auditlog/live` (real-time dashboard) · **Go**: 1.26+ · **Status**: ALPHA

---

## Commands

| Command                                                                             | Purpose                                                                                                      |
| ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `GOEXPERIMENT=jsonv2 go test ./...`                                                 | Run core tests (requires jsonv2)                                                                             |
| `GOEXPERIMENT=jsonv2 go test -race ./...`                                           | Run core tests with race detector                                                                            |
| `GOEXPERIMENT=jsonv2 go test -race -coverprofile=cover.out -covermode=atomic ./...` | Core tests with coverage (~95.3%)                                                                            |
| `GOEXPERIMENT=jsonv2 go vet ./...`                                                  | Core static analysis                                                                                         |
| `golangci-lint run ./...`                                                           | Lint core (golangci-lint v2, 0 issues)                                                                       |
| `cd viz && GOEXPERIMENT=jsonv2 go test ./...`                                       | Run viz tests (requires jsonv2)                                                                              |
| `cd viz && GOWORK=off GOEXPERIMENT=jsonv2 go test ./...`                            | Run viz tests in standalone mode (no workspace)                                                              |
| `cd viz && GOEXPERIMENT=jsonv2 go vet ./...`                                        | Viz static analysis                                                                                          |
| `cd viz && golangci-lint run ./...`                                                 | Lint viz                                                                                                     |
| `cd live && GOEXPERIMENT=jsonv2 go test ./...`                                      | Run live tests (requires jsonv2)                                                                             |
| `cd live && GOEXPERIMENT=jsonv2 go vet ./...`                                       | Live static analysis                                                                                         |
| `cd live && golangci-lint run ./...`                                                | Lint live                                                                                                    |
| `cd live && GOEXPERIMENT=jsonv2 go run ./demo`                                      | Run live dashboard demo (http://localhost:18080)                                                             |
| `go run ./viz/example`                                                              | Run the demo pipeline                                                                                        |
| `go run ./example`                                                                  | Run the demo pipeline (legacy path; now `./viz/example`)                                                     |
| `nix run .#check`                                                                   | Run all checks (vet + test-race + lint + govulncheck for core, viz & live — all three modules fully covered) |
| `go work sync`                                                                      | Sync `go.work` and `go.work.sum` with module state                                                           |

---

## Architecture

The project is split into three Go modules:

1. **Core module** (`github.com/larsartmann/go-workflow-auditlog`, package `auditlog`) —
   event recording, report construction, JSON/NDJSON serialization, replay, diff, filter, and index.
   It has no dependency on `go-output`.
2. **Visualization module** (`github.com/larsartmann/go-workflow-auditlog/viz`, package `viz`) —
   diagrams (Mermaid, PlantUML, Graphviz DOT, D2), tables (16 formats), trees (ASCII/HTML), and the
   interactive HTML dashboard. It depends on the core module and on `github.com/larsartmann/go-output`.
3. **Live dashboard module** (`github.com/larsartmann/go-workflow-auditlog/live`, package `live`) —
   real-time HTTP dashboard with Server-Sent Events (SSE) streaming. Steps light up as they execute,
   the DAG graph snaps into place on completion. Depends on both core and viz.

### Shared infrastructure: `go-sse`

The `live/` module depends on [`github.com/larsartmann/go-sse`](https://github.com/larsartmann/go-sse)
v0.4.0 (public, pinned) for SSE transport primitives. The live module uses:

- `sse.Stream` — replaces manual SSE plumbing in `handleSSE` (headers, flusher,
  heartbeat, `WriteEvent`+`Flush`). `sse.NewStream(w, r)` sets the required
  SSE headers and writes 200 OK. `stream.Send(evt)` serializes and flushes.
  `go stream.Heartbeat(ctx, interval)` sends keepalive comment frames.
- `sse.Replay` + `sse.EventStore` — reconnection replay. The Hub maintains a
  bounded event ring buffer (`replay.go`) that implements `sse.EventStore`.
  When a client reconnects with `Last-Event-ID`, `sse.Replay(stream, store,
lastID)` replays missed events before the snapshot.
- `sse.Event`, `sse.EventID`, `sse.WriteEvent`, `sse.ContentType` — wire-format
  primitives used by both Stream and direct calls.

The domain-specific Hub, Server, ring buffer, and drain logic are implemented
locally in `live/` on top of these primitives; go-sse itself is transport-only
and owns no domain types here.

A `go.work` workspace links only the project's own modules (core `.`, `./viz`, `./live`)
for local development; **all external `larsartmann/*` modules are resolved from published
versions** (no local `use`/`replace` for them): `go-output` v0.35.0 + its sub-modules
(`viz`, `live`), `go-sse` v0.4.0 (`live`), `go-atomic-write` v0.4.1,
`go-error-family` v0.10.0, and `go-ndjson` v0.0.1 (`core`). `go.work` and `go.work.sum` are
gitignored (local dev artifacts). Standalone (`GOWORK=off`) builds work for all three
modules because every dependency has a real, checksum-verified version in `go.sum`.
**Note:** `go work sync` prints harmless `downloading ... go-output/testhelpers
v0.0.0-00010101000000-000000000000` lines because the _published_
`go-output@v0.31.1/go.mod` contains local `replace` directives (`=> ./testhelpers`) that Go
ignores when consuming it as a dependency — builds/tests are unaffected. **However**, the same
defect breaks `go mod tidy` on the `viz` and `live` sub-modules in standalone mode: tidy tries
to resolve the test graph and exits 1. The workaround is `go mod tidy -e` (error-tolerant),
which proceeds past the unresolvable testhelpers while still tidying everything else and
catching real `go.sum` drift. This is applied in CI (`.github/workflows/ci.yml` `mod-tidy`
job) and in `.goreleaser.yml` `before:` hooks. go-output v0.32.0 still has the same replace
directives, so the defect is not yet fixed upstream.

### Core module source files

```
doc.go             — Package doc comment
types.go           — Domain enums: EventType, Phase, StepStatus, StepRef, flowStatusMap, fromFlowStatus, SchemaVersion, AllStepStatuses, AllEventTypes
event.go           — Event type (embeds StepRef, carries RunID) + convenience methods
step.go            — StepInfo type + stepCore (shared accumulator for live/replay) + toStepInfo() conversion
plugin.go          — Public API: New(), Attach(), Snapshot(), Report(), WriteJSON/WriteNDJSON/ExportJSON/ExportNDJSON, Config, Auditor, RunID() + ErrExportWriteFailed sentinel
recorder.go        — Core state machine: event capture, step records, attempt tracking, RunID + StepID counters
runid.go           — newRunID(): 128-bit crypto-random hex run identifier
attach.go          — Attach/Snapshot/CaptureDAG logic: callback injection + post-run DAG capture (incl. sub-workflows) + pre-execution DAG structure population
report.go          — WorkflowReport type (carries RunID) + Validate() + sentinel errors (incl. ErrRenderFailed) + query methods + Duration()/Summary()/CriticalPath()/PeakConcurrencySteps() + stepsByName + WriteJSON/WriteNDJSON + ExportJSON/ExportNDJSON + computeWallClockDurationMs
report_builder.go  — BuildReport assembly: step records → sorted StepInfo + aggregates (WorkflowSucceeded, finalizeDenormalized) + computeCriticalPath/computePeakConcurrency/computePeakConcurrencySteps + sortEventsByTime
filter.go          — Report filtering (Filtered, ReportOption, WithStepsByStatus, etc.)
diff.go            — Diff API: DiffResult/StepDiff, Diff() between reports
index.go           — ReportIndex: opt-in O(1) lookup maps over a report
loader.go          — LoadReport / LoadReportFromReader / LoadReportFromBytes + ErrReportLoadFailed sentinel
export.go          — NDJSON writer (writeEventsNDJSON + encodeEvent shared helper used by both export and stream paths)
ndjson.go          — ReadEvents NDJSON reader (sentinel errors, enum validation on ingest)
replay.go          — ReplayEvents: reconstruct Report from event stream (uses stepCore from step.go, preserves RunID + assigns StepIDs)
stream.go          — NDJSONStreamer: real-time streaming NDJSON writer (thread-safe OnEvent callback, WithAutoFlush, WithBufferSize, CreateNDJSONStreamer)
classify.go        — Error classification: RegisterClassifications() + ErrorClassifications() map sentinel errors → go-error-family Family
csv.go             — WriteCSV/WriteTSV/ExportCSV/ExportTSV: delimited-value export of all steps (stdlib encoding/csv, pointer fields as empty strings)
helpers.go         — Utility helpers: CheckNoClobber, HasPointerAddress, NameCollisions + ErrFileExists sentinel + WriteToFile (atomic temp+rename export helper)
testhelpers/     — Exported test fixtures, step constructors, assertions, and FailingWriter/ErrWriteFailed shared by both modules
```

### Visualization module source files

```
viz.go              — Package doc + type aliases (WorkflowReport, StepInfo, StepStatus, etc.) and re-exports (ErrRenderFailed, ErrExportWriteFailed, WriteToFile, AllStepStatuses, AllEventTypes)
diagram.go          — Translation layer: buildGraph() converts WorkflowReport → go-output GraphNode/GraphEdge + statusStyle() + stepLabel()
diagram_options.go  — DiagramOption type + WithDirection(output.Direction) + per-format direction helpers
render.go           — Shared render helpers: writeRendered, writeRenderedTransformed, writeGraph
mermaid.go          — WriteMermaid, WriteMermaidString, ExportMermaid
plantuml.go         — WritePlantUML, WritePlantUMLString, ExportPlantUML
graphviz.go         — WriteGraphviz, WriteGraphvizString, ExportGraphviz
d2.go               — WriteD2, WriteD2String, ExportD2
daghtml_adapter.go  — buildDAGHTML() for the interactive HTML graph renderer + humanizeMs() for compact node duration labels
metadata.go         — TypeMetadata struct + BuildTypeMetadata() for the HTML dashboard JS
table.go            — WriteTable, WriteTableString, ExportTable
table_options.go    — TableColumn enum + WithColumns + DefaultTableColumns + AllTableColumns
tree.go             — WriteTree, WriteTreeString, ExportTree, WriteHTMLTree, WriteHTMLTreeString, ExportHTMLTree
html.go             — WriteHTML, WriteHTMLString, ExportHTML
html_render.go      — renderHTML(): assemble self-contained HTML dashboard
dashboard.css       — Dashboard CSS theme (embedded via go:embed)
dashboard.js        — Dashboard JavaScript: tabs, tables, Gantt timeline, tree, graph enhancements (critical path, retry badges, search, duration labels)
example/            — Data pipeline demo (now in viz module)
```

### Data Flow

1. User creates `Auditor` via `auditlog.New(Config)` — optionally passing `NDJSONStreamer.OnEvent` as `Config.OnEvent` for real-time streaming
2. `Attach(w)` injects `BeforeStep`/`AfterStep` callbacks into all steps via `State.MergeConfig`
3. Optionally `CaptureDAG(w)` pre-populates step records with names, types, dependencies, retry/timeout config, and a "pending" status — makes DAG available BEFORE `Do(ctx)` for live dashboards
4. During `w.Do(ctx)`, callbacks fire per-attempt → `Recorder` captures timestamped `Event`s → `OnEvent` (if set) fires outside the lock (e.g. streaming to NDJSON)
5. `Snapshot(w)` reads `w.StateOf(step)` + `w.UpstreamOf(step)` to fill in DAG structure and skipped/canceled statuses
6. `Report()` assembles `StepInfo` slice (with forward + reverse deps) and event stream
7. Core consumers call `report.WriteJSON` / `report.WriteNDJSON` / `report.ExportJSON` / `report.ExportNDJSON`
8. Consumers who want visualization import `viz` and call `viz.WriteX(report, ...)` or `viz.ExportX(report, ...)`
9. Consumers who want real-time monitoring import `live`, wire `hub.OnEvent` as `Config.OnEvent`, serve the dashboard, and call `server.SignalComplete()` after `Snapshot(w)`

### Live module source files

```
doc.go             — Package doc comment
hub.go             — Hub: SSE subscriber registry, fan-out OnEvent (assigns SSE event IDs, stores in ring buffer), SignalComplete, Drain (graceful shutdown), non-blocking broadcast
replay.go          — eventRingBuffer: bounded thread-safe ring buffer implementing sse.EventStore for reconnection replay
server.go          — HTTP server: SSE handler (uses sse.Stream + sse.Replay), /api/report, /api/health (with drain/buffer state), /api/export/ndjson, /api/export/html, dashboard serving, configurable Prefix, CORS middleware (secure-by-default: empty=disabled), New() convenience, ServeHTTP, graceful Shutdown (drains subscriber buffers first)
dashboard.go       — HTML template assembly: reuses viz CSS + embeds live CSS + JS + daghtml graph JS; skip link, ARIA landmarks, focusable sortable headers, help modal, export buttons (JSON/NDJSON/HTML) in header with ROUTE_PREFIX-aware URLs
dashboard.css      — Live-specific CSS: pulsing live badge, connection status, step animations, graph placeholders, export button styles, :focus-visible rings, sort-direction indicators, help modal, skip link
dashboard.js       — SSE client (EventSource with reconnection replay via Last-Event-ID) + incremental rendering engine: state management, requestAnimationFrame batching, diff-based steps table (no innerHTML rebuild), live graph node updates (colors + duration labels), critical path/retry badges/search, minimap with MutationObserver viewport tracking, global keyboard shortcuts, graph and step-row keyboard navigation, accessible help modal
demo/              — Demo pipeline with retry: fetch → validate → transform/enrich → save (flaky, retries) → notify
```

### Live Data Flow (SSE)

1. User creates server + auditor via `live.New(auditlog.Config{...}, live.Config{Addr: ":8080"})`
2. `hub.OnEvent` is wired as the auditlog `Config.OnEvent` callback
3. `auditor.Attach(w)` + `auditor.CaptureDAG(w)` + `go server.ListenAndServe()` starts the dashboard
4. Browser connects to `/api/events` → receives `snapshot` event (current report + events + metadata + DAG) — the DAG is immediately available because `CaptureDAG` pre-populated step structure
5. As `w.Do(ctx)` executes, each captured event is fanned out via SSE `event` messages — graph nodes update colors in real-time via `updateGraphLive()`
6. Browser incrementally renders: new steps appear, running steps animate, statuses change with flash effects
7. After `auditor.Snapshot(w)` + `server.SignalComplete()`, all clients receive a `complete` event with the final report + full DAG
8. The DAG graph tab activates with the Sugiyama layout from daghtml, showing the complete dependency structure

### Keyboard Navigation (live dashboard)

The live dashboard is operable without a mouse. All keyboard shortcuts are suppressed while focus is inside an `<input>`, `<textarea>`, or `<select>`.

- **Global shortcuts** — `1`–`4` switch tabs (Steps, Graph, Timeline, Events); `/` focuses the step search; `g` focuses the graph search; `e` toggles errors-only; `c` toggles critical-path highlight; `f` fits the graph; `+`/`=` zooms in, `-` zooms out; `x` expands/collapses the step list; `?` opens the shortcut help modal; `Esc` closes the modal, error tooltip, or help.
- **Tab bar** — Arrow keys move focus between tabs; `Home`/`End` jump to first/last tab; activation switches the visible tabpanel and moves focus into it.
- **Sortable headers** — `Tab` reaches each header; `Enter`/`Space` activates sorting; `aria-sort` reflects the current direction.
- **Step table rows** — The first visible row is in the tab order (`tabindex="0"`), others are `tabindex="-1"`. Arrow `Up`/`Down` move between rows; `Home`/`End` jump to first/last; `Enter`/`Space` on a row with an error opens the error tooltip; `Esc` closes it.
- **Graph nodes** — Nodes are focusable SVG groups (`tabindex="0"`, `role="button"`, `aria-label` includes name, status, and duration). Arrow keys move to connected neighbors; `Enter`/`Space` jumps to the matching step row in the Steps tab.
- **Landmarks** — Skip-to-main link, `<header role="banner">`, `<nav role="navigation">` around the tab bar, `<main id="main-content" role="main">` around tab panels, and `aria-live` regions on the live badge, connection status, stats, and result counters.

### Concurrency Model

- **Single `sync.RWMutex` (`mu`)** protects all mutable state: `events`, `steps`, `stepCounter`.
- `sequence` is `atomic.Int64` — no mutex needed for the counter.
- `stepCounter` (for `StepID`) is a plain int guarded by `mu` (assigned under the write lock).
- Each callback acquires `mu` once, performs all mutations, then releases.
- `onEvent` callback is always called outside the lock to avoid blocking. **It fires
  concurrently** from parallel step goroutines — consumers must be goroutine-safe,
  and delivery order is not guaranteed to match event Sequence (sort if needed).
- `BuildReport()` uses `mu.RLock()` for reading.

---

## Integration Model

go-workflow v0.1.13 provides **no interceptors** (unlike what some docs claim). The extension points are:

1. **`BeforeStep` / `AfterStep` callbacks** — per-step, per-attempt. Configured via `.BeforeStep()` / `.AfterStep()` on the builder, or injected post-Add via `State.MergeConfig`.
2. **`Workflow.StateOf(step)`** — returns `*State` with status, error, config.
3. **`Workflow.UpstreamOf(step)`** — returns the direct upstream steps.

Our `Attach(w)` iterates `w.Steps()` (root steps) and calls `state.MergeConfig()` to inject audit callbacks. These fire per-attempt during execution.

### Why Snapshot is needed

Steps settled inline by Conditions (Skipped/Canceled) **bypass** the Before/After callback chain entirely. `Snapshot(w)` reads `w.StateOf(step).GetStatus()` to capture their final status. It also reads the full DAG structure and retry/timeout config.

### Critical: BeforeStep must pass through context

The `BeforeStep` callback signature is `func(ctx, Steper) (context.Context, error)`. The returned context flows into `step.Do(ctx)`. If the callback returns `context.Background()`, **step-level timeouts are destroyed**. Our implementation returns the original `ctx` unchanged.

---

## Gotchas

- **Linter config: `nlreturn` was removed** (wsl_v5 retained). `.golangci.yml` previously enabled BOTH `nlreturn` and `wsl_v5`, which directly contradict each other on blank-line-before-return, forcing a `//nolint:nlreturn` workaround in the live module. `wsl_v5` provides comprehensive whitespace control that fully subsumes `nlreturn`'s single rule; the workaround was removed. Do NOT re-enable `nlreturn`. All three modules lint clean.
- **GO-2026-5856 (crypto/tls ECH privacy leak) is now FIXED.** All three `go.mod` files + `go.work` carry `go 1.26.5`, and `nixos-unstable` now ships `go_1_26` at 1.26.5, so the fix is available in the nix toolchain. `govulncheck` for the live module (previously omitted because `live.Server.ListenAndServe` sat on the affected call path under 1.26.4) is now **re-enabled** in `nix run .#check` and passes with no findings. (History: it was deferred from the v0.8.0 module split until nixpkgs bumped past 1.26.4; resolved in v0.8.1.)
- **go-workflow v0.1.13 has a data race** in `DefaultRetryOption.Backoff` — the shared `ExponentialBackOff` instance races when used concurrently. Tests must create fresh backoff instances: `o.Backoff = backoff.NewExponentialBackOff()`.
- **`flow.String(step)`** returns `*TypeName(0xpointer)` by default — non-deterministic across runs. Users should implement `String()` on step types or use `flow.Name()` for clean audit output.
- **Step names may collide** if two steps have the same `String()` output. Step identity in the recorder uses the `flow.Steper` pointer (which IS unique/comparable), so internal tracking is correct. Only the JSON `step_name` field may be ambiguous. Documented as a known limitation.
- **`flow.StepStatus` uses capitalized strings** ("Succeeded", "Failed", etc.) while our JSON uses lowercase ("succeeded", "failed"). Conversion happens in `fromFlowStatus()`.
- **`RetryOption.Attempts`** (not `MaxAttempts`) is the field name in go-workflow v0.1.13. The source doc comment says `MaxAttempts` but the actual struct field is `Attempts`.
- **`Pipe` only sets dependencies**, not data flow. Data flows via `.Input()` callbacks. The example wires both.
- **Sub-workflow traversal**: `snapshotWorkflow` uses `flow.Traverse` to walk the full step DAG, capturing inner steps of composite/sub-workflows that bypass Before/After callbacks. Wrapper steps with nil `StateOf` are skipped via `TraverseEndBranch`.
- **`buildReportFromCore()` is the single Report construction path** — `BuildReport`, `Filtered`, and `ReplayEvents` all route through it. The denormalized aggregate fields are derived in exactly one place. Any new construction path MUST use it.
- **Three duration metrics exist** — `TotalDurationMs` (sum of all per-step durations; inflated for parallel workflows), `WallClockDurationMs` (actual elapsed time from earliest to latest event; the "how long did I wait" number), and `CriticalPathDurationMs` (longest dependency-chain duration; the bottleneck path). Always use `WallClockDurationMs` for user-facing summaries and diff/regression detection. `Duration()` in `report.go` delegates to `computeWallClockDurationMs()` (same logic as the JSON field).
- **WriteToFile** (in `helpers.go`) is atomic (temp file + rename + bufio). It is exported so the `viz` module can reuse it for all diagram/table/tree/HTML exports. A crash during export leaves the previous file intact, not a partial write.
- **NDJSON reader** validates enums on ingest: events with unknown `event_type` or `phase` are rejected with a descriptive error. Whitespace-only lines are skipped (not just empty lines).
- **Streaming NDJSON** (`NDJSONStreamer` in `stream.go`) writes events as NDJSON in real time via the `Config.OnEvent` callback, without buffering the entire run in memory first. Thread-safe (mutex-protected writes since `OnEvent` fires concurrently from parallel step goroutines). 64 KB internal buffer by default (configurable via `WithBufferSize`); call `Flush()` or `Close()` to guarantee all data is written. `WithAutoFlush()` flushes after every event (for real-time tailing). `CreateNDJSONStreamer(path)` is a file convenience constructor (writes directly — NOT atomic, by design, so consumers can tail the file live). Errors are first-error-wins: `Err()` returns the first write failure, subsequent events are silently dropped. Output is compatible with `ReadEvents` for round-trip. Event ordering in the file may differ from `Sequence` order (concurrent steps fire `OnEvent` from different goroutines) — consumers sort by `Sequence` if needed. **MaxEvents interaction**: `OnEvent` fires for ALL captured events, including those dropped by `Config.MaxEvents` (the callback fires outside the lock, after `appendEventLocked` which may skip storage). A streamer therefore sees more events than the recorder stores — this is intentional, since streaming is for external consumers who want every event regardless of in-memory caps.
- **Diagram/table/tree/HTML exports** live in the `viz` module and use [go-output](https://github.com/larsartmann/go-output) renderers. `buildGraph()` in `viz/diagram.go` translates `WorkflowReport` steps into `output.GraphNode` + `output.GraphEdge` (edges point dependency → step, following execution flow — matching the tree export and the GitHub Actions / Airflow convention). `statusStyle()` maps `StepStatus` → `output.NodeStyle` fill colors (`succeeded`=`#2d5a2d` green, `failed`=`#8b2d2d` red, `skipped`=`#4a4a4a` gray, `canceled`=`#5a3d2d` orange). Mermaid uses per-node `style` directives (not `classDef`); code fence is OFF for raw `.mmd` output. DOT uses `graphID "workflow"`. PlantUML uses `[label] as id` component notation. D2 uses inline `style.fill`/`style.font-color` on nodes with `title: { label: Workflow DAG }`. Each renderer handles its own ID sanitization and label escaping — auditlog passes raw step names as node IDs. **Layout direction** (`viz.WithDirection(output.Direction)`) is supported on all 4 diagram formats: DOT/D2 use native go-output `SetDirection()`; Mermaid post-processes `flowchart TD` → `flowchart LR/BT/RL`; PlantUML injects `left to right direction`. Direction helpers in `viz/diagram_options.go`. Dependency: `go-output` root v0.31.1 + sub-modules at v0.31.1 (graph, plantuml, d2, daghtml, tree, table, markdown, markup, delimited, serialization; multi-module; all aligned). **D2 quoting fix**: go-output v0.31.1 (published and pushed to remote) adds `d2Quote()` which wraps hex colors (`#2d5a2d`) and labels with spaces/brackets (`fetch [Succeeded]`) in double quotes — D2 treats `#` as a comment character. The fix also quotes DOT edge `color` attributes. Mermaid and PlantUML were not affected (Mermaid sanitizes IDs to alphanumeric; PlantUML uses `#` as its color prefix). The `.golangci.yml` exhaustruct exclude list must match the actual go-output struct names (`d2.Node`, `d2.NodeStyle`, `d2.StrokeStyle`, `d2.Edge`, `output.NodeStyle`) — not the old prefixed names (`D2Node` etc.). `github.com/larsartmann/go-sse.Event` is also excluded (the SSE event struct has optional `ID`/`Retry` fields that are intentionally omitted). **treefmt** config in `flake.nix` includes `d2-fmt` (via `settings.formatter.d2`) for `.d2` file formatting, plus `nixfmt` and `gofmt`.
- **Table export** (`viz.WriteTable`) delegates to `output.RenderTableData` which dispatches to 16+ registered formats: table, json, csv, tsv, markdown, xml, d2, yaml, html, tree, mermaid, dot, jsonl, asciidoc, toml, plantuml. Sub-module imports auto-register renderers via `init()`. **Column selection** via `viz.WithColumns(viz.TableColumn...)` — 10 columns available (Step, Status, Duration, Attempts, MaxAttempts, Retry, Timeout, Error, Type, Dependencies). `columnDefs` in `viz/table_options.go` is the single source of truth mapping `TableColumn` → header + extractor. Default = original 7 columns (backward compatible). Empty `WithColumns()` or no option → defaults.
- **Tree export** (`viz.WriteTree` / `viz.WriteHTMLTree`) builds a `TreeNode` forest from step DAG: root = steps with no dependencies, children = dependents (execution flow). `tree.ASCIITreeRenderer` produces depth-colored ASCII output; `markup.HTMLTreeRenderer` produces nested `<ul>` lists.
- **`StepInfo.Error` reflects the FINAL outcome only.** For a step that fails on attempts 1–2 and succeeds on attempt 3, the `Error` field is `nil` (not "transient failure" from the last failed attempt). The per-attempt error history is preserved in the `Event` stream — each `attempt_end` event carries its own `Error`. Rationale: the step-level `Error` is the answer to "why did this step end in its final state?", and a succeeded step ended successfully. See `recorder.go:recordAfterStep` and the regression test `TestRetry_StepErrorClearedOnSuccess`.
- **`RunID` is a branded string type** (`type RunID string`) defined in `types.go`. It serializes to/from JSON as a plain string but the type system prevents accidentally passing a `WorkflowID` (also a string) where a `RunID` is expected. Convert with `RunID("value")` or `string(id)`. The `len()` built-in works directly on `RunID`; `hex.DecodeString` and similar stdlib functions need `string(runID)`.
- **`stepCore` is the shared step-state accumulator** (`step.go`) embedded by both `stepRecord` (live capture, `recorder.go`) and the replay accumulator (`replay.go`). The single `toStepInfo()` method produces the public `StepInfo` from the common fields. Live-only fields (stepID, pendingAttempts, maxAttempts, hasRetry, hasTimeout, dependencies) live on `stepRecord` only. Any new step-state field MUST go on `stepCore` so both paths stay synchronized.
- **HTML dashboard** (`viz/html_render.go`) uses `go:embed` to embed `viz/dashboard.css` and `viz/dashboard.js` as separate files (proper syntax highlighting + linting). The `renderHTML()` function uses `fmt.Sprintf` on the `htmlTemplate` const (eight `%s` verbs: CSS, version, report JSON, metadata JSON, DAG data JSON, dashboard JS, daghtml graph JS, version). Report data is injected via `<script type="application/json">` tags (never parsed as HTML by the browser), and dynamic content in JS is escaped via the `esc()` function. Strict CSP: `default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'`. The daghtml SDK (`go-output/daghtml` v0.31.1) provides the SVG graph renderer with Sugiyama layout (rank assignment via Kahn's, 4-pass barycenter crossing reduction, median-alignment positioning, pan/zoom/touch/click-highlight) — invoked via `initDAGGraph(containerId, dataScriptId)` in the embedded daghtml JS. `dashboard.js` post-processes the daghtml SVG in `enhanceGraph()` to add: critical path highlighting (toggle button, memoized DFS mirroring Go's `computeCriticalPath`), retry count badges (`↻N` amber circles), and graph search/filter. **`enhanceGraph()` is idempotent** — guarded by `container.dataset.enhanced` to prevent duplicate badges/listeners on repeated tab switches. `viz.BuildTypeMetadata()` in `viz/metadata.go` is the single source of truth for enum display metadata consumed by the JS. JS status colors reference CSS variables (`var(--success)` etc.) — no hardcoded hex duplication between CSS and JS. The diagram export hex colors in `types.go` (`stepStatusMeta`) are separate because diagram formats (Mermaid/DOT/D2) require literal hex values, not CSS variables. Node duration labels use `humanizeMs()` in `daghtml_adapter.go` (compact: `<1ms`, `Nms`, `N.Xs`) while the rest of the dashboard uses `humanizeDuration()` in `dashboard.js` (verbose: `48.8s`, `5m 18s`) — different formats are intentional. Tree root detection logic exists in both `viz/tree.go` (Go, server-side tree export) and `dashboard.js` (client-side tree tab) — this is intentional: Go renders the tree export for static files, JS renders the interactive tab. Golden content test (`TestReport_WriteHTML_GoldenContent` in `viz/html_golden_test.go`) validates structural + semantic content (DOCTYPE, CSP, 5 script tags, 5 tab panels, step names, WorkflowID, RunID, embedded CSS/JS, strict CSP policy, graph enhancement markers) — NOT byte-for-byte comparison, which was retired after breaking 6+ times on whitespace/dependency drift.
- **Error classification** (`classify.go`) registers all sentinel errors with [go-error-family](https://github.com/larsartmann/go-error-family) via `init()` auto-registration into `DefaultRegistry`. Consumers importing auditlog automatically get `errorfamily.Classify(err)`, `errorfamily.IsRetryable(err)`, and `errorfamily.ExitCode(err)` on auditlog errors. Strategy A (registration, not replacement) — sentinels stay as plain `error` values; `errors.Is` semantics are unchanged. Mapping: **Corruption** (exit 65, not retryable) = `ErrEventCountMismatch`, `ErrStepCountMismatch`, `ErrStatusDrift`, `ErrCountMismatch` (data integrity violations). **Rejection** (exit 1, not retryable) = `ErrEmpty`, `ErrNoEvents`, `ErrOversizedLine`, `ErrWorkflowIDPathSep`, `ErrReplayNoEvents` (bad caller input). **Transient** (exit 75, retryable) = `ErrReportLoadFailed` (retryable load/decode failures). **Infrastructure** (exit 69, not retryable) = `ErrRenderFailed` (rendering/marshaling), `ErrExportWriteFailed` (file write/flush/rename). Private sentinels `errUnknownEventType`/`errUnknownPhase` are also registered as Rejection. All 24 I/O error paths are wrapped with sentinels (loader, plugin/WriteToFile, export, report.WriteJSON, csv header/step/flush, viz diagram/table/tree/html render+write). `ErrorClassifications()` returns the canonical map for consumer-side custom registries. `RegisterClassifications(*Registry)` registers into a custom registry for test isolation or scoped overrides. Unregistered errors default to Transient (fail-open for retry).
- **Critical path steps are injected from Go into report JSON.** The Go implementation (`computeCriticalPath()` in `report_builder.go`) computes the memoized DFS and populates `CriticalPathSteps []string` on the report (serialized as `critical_path_steps`). The JS implementation (`computeCriticalPathSteps()` in `dashboard.js`) uses the injected field directly, falling back to a client-side DFS for older reports without the field. The duplication is now a graceful-degradation fallback, not a maintenance burden — changing the Go algorithm automatically updates the dashboard for new reports.
- **`enhanceGraph()` is coupled to daghtml DOM internals.** It reads `dataset.id`, `dataset.source`, `dataset.target` on SVG elements produced by the daghtml SDK's `initDAGGraph()`. If daghtml changes its DOM structure or data attributes, the post-processing (retry badges, critical path, search, edge coloring, node click navigation) breaks silently. The function is idempotent (guarded by `container.dataset.enhanced`), so switching graph tabs multiple times is safe.
- **Module split:** Core (`github.com/larsartmann/go-workflow-auditlog`), visualization
  (`github.com/larsartmann/go-workflow-auditlog/viz`), and live
  (`github.com/larsartmann/go-workflow-auditlog/live`) are separate Go modules. The core
  module has no `go-output` dependency. All three modules share a `go.work` workspace in
  development; CI also verifies `GOWORK=off` standalone builds. The `testhelpers` package
  lives inside the core module so both core and viz tests can import it without a circular
  module dependency.
- **Release tagging convention:** Each release produces **three annotated tags** at the same
  commit: `vX.Y.Z` (core), `viz/vX.Y.Z` (viz), `live/vX.Y.Z` (live). The path prefixes are
  required by the Go module system so `go get` resolves each module independently. The
  full release process is documented in [`RELEASE.md`](RELEASE.md) — **read it before
  cutting a release.** The previous ad-hoc release (v0.8.1) bypassed the documented process
  and is the reason RELEASE.md now exists.
- **goreleaser multi-module gotchas:** `.goreleaser.yml` releases the core module only (the
  GitHub Release + demo binary represent the whole monorepo). Three things are required:
  (1) `GORELEASER_CURRENT_TAG=vX.Y.Z` must be set — three tags share one commit and
  goreleaser's `git describe` picks the alphabetically-last (`live/v*`) without it.
  (2) Before-hooks are wrapped in `sh -c "..."` because goreleaser OSS uses direct
  `exec.CommandContext` (not a shell) — inline env vars (`FOO=bar cmd`) and shell builtins
  (`cd`, `&&`) silently fail without the wrapper. (3) A **clean working tree** is required
  (the auto-commit daemon must have committed all pending changes). If the tree is dirty,
  fall back to `gh release create` + manual binary upload (see RELEASE.md).
- **CI `mod-tidy` job** uses `go mod tidy -e` for viz/live (see go-output testhelpers defect
  above). Core uses plain `go mod tidy`. The drift check (`git diff --exit-code`) still
  catches real `go.sum` skew.
- **CRITICAL — sub-module `go.mod` files must NOT contain `replace` directives for sibling
  modules.** The v0.8.0 module split left `replace github.com/larsartmann/go-workflow-auditlog
=> ..` in `viz/go.mod` and `live/go.mod`. These local filesystem redirects work in
  `go.work` workspace mode but produce invalid pseudo-version requirements
  (`v0.0.0-00010101000000-000000000000`) that **completely break consumer `go get`** for
  viz and live when fetched in isolation. Local development is handled by `go.work`'s `use`
  directives — `replace` directives in sub-module `go.mod` files are redundant and harmful.
  Fixed in v0.8.2. **Pre-release check:** `grep -r '^replace' viz/go.mod live/go.mod` must
  return nothing. See RELEASE.md.

---

## Testing Patterns

- Standard `testing.T` + table-driven tests. No ginkgo/testify.
- Test steps implement `String()` for deterministic names.
- Retry tests use fresh `backoff.NewExponentialBackOff()` to avoid the go-workflow race.
- External test packages: `auditlog_test` for core, `viz_test` for visualization.
- Shared test helpers live in `testhelpers` (exported package inside the core module) and are imported by both core and viz tests.
- `t.Setenv()` for env var tests (runs sequentially).
- 459 test functions across 3 modules (core: 165; viz: 229; live: 65) covering: disabled/enabled, success/failure, dependencies, retry, timeout/cancel, skip, concurrent steps, fan-out/fan-in, event ordering, OnEvent callback, **CaptureDAG** (pre-execution DAG population, idempotent, disabled no-op, nil workflow safety, sub-workflow traversal, retry config capture, no events generated), export formats (JSON/NDJSON/D2/table/tree/HTML), streaming NDJSON (NDJSONStreamer real-time, concurrent safety 16 goroutines, auto-flush, WithBufferSize, encode-error path, flush/Close error paths, round-trip, workflow integration, 100% stream.go coverage), report validation, query methods, filter (combined type+time interaction), diff, replay, load, diagrams (Mermaid/PlantUML/DOT/D2), diagram direction (TD/LR/BT/RL across 4 formats), table column configuration (10 columns, custom selection, ordering, replayed reports), edge-direction consistency (diagrams vs tree), API symmetry (Write\*String functions in `viz`, Export\* functions in `viz`), high-fan-out peak concurrency, diamond-DAG critical path, CriticalPath() step chain, PeakConcurrencySteps(), HTML from replayed/loaded reports, HTML diamond DAG + high fan-out, HTML structural integrity, HTML golden content validation, HTML determinism, edge cases, error classification (all 12 public sentinels mapped to Family, wrapped error chain classification, custom registry registration, ExitCode/IsRetryable behavior, errors.Is identity preserved), **error-path tests** (failing io.Writer injection into all `viz.Write*` methods, unwritable directories for `viz.Export*`, invalid input for `Load*` — verifying errors.Is matches `ErrRenderFailed`/`ErrExportWriteFailed`/`ErrReportLoadFailed` on all wrapped paths), plus regression tests for fixed bugs (status drift, diff ordering, NDJSON line numbers, WorkflowSucceeded honesty about pending steps, stale error cleared on retry success), **fuzz tests** (diagram special-char injection across Mermaid/PlantUML/DOT/D2; multi-step diagram sanitization with unicode/control-char/keyword-collision seeds; HTML XSS injection with structural integrity checks; Classify adversarial wrapped error chains), **property-based tests** (Diff algebra: identity, added/removed duality, duration anti-symmetry, status-change symmetry, sorted output — 200 iterations each, deterministic seeds; Classify wrapping-preserves-family through arbitrary depth, Classify identity matches ErrorClassifications map — 200 iterations), and **benchmarks** (runtime overhead: Invocation, Attach, BuildReport, EventsCopy, OnEventCallback, RetryWithAudit, MermaidExport; export rendering: viz.WriteD2/viz.WriteTable/viz.WriteTree/viz.WriteJSON/viz.WriteMermaid on 100-step reports; renderHTML small 3-step + large 1000-step reports; godoc examples: Duration, Filtered, PeakConcurrency, CriticalPathDurationMs, WallClockDurationMs).
- Coverage: **~95.3%** of statements (auditlog package), **91.7%** viz, **95.5%** live.
- **Fuzz targets**: `FuzzDiagramSpecialChars` — diagram export structural integrity against injection payloads; `FuzzDiagramSanitization_MultiStep` — multi-step diagram sanitization (17 seed pairs: unicode/emoji/CJK/Arabic, control chars, diagram-keyword collisions, whitespace-only, length extremes, edge sanitization) across Mermaid/PlantUML/DOT/D2; `FuzzHTMLSpecialChars` — HTML dashboard XSS containment (12 seed payloads via step names, errors, dependency names) + structural integrity validation (balanced script tags, DOCTYPE, CSP).
- **Property tests**: 5 Diff algebra properties with 200 random report pairs each; HTML determinism (same report → identical output).
- **Benchmarks**: `BenchmarkRenderHTML_LargeReport` (1000 steps) + `BenchmarkRenderHTML_SmallReport` (3 steps) + `BenchmarkNDJSONStreamer_{100,1000,10000}Events` (streaming throughput to io.Discard) + `BenchmarkWriteCSV_LargeReport` (100 steps, core CSV export).

### Shared test helpers (in `testhelpers` package)

| Helper                                             | Purpose                                                                                                    |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `NewSucceed`, `NewFail`, `NewFlaky`, `NewSlow`     | Construct test step instances                                                                              |
| `SucceedStep`, `FailStep`, `FlakyStep`, `SlowStep` | Test step types exported for direct use in tests                                                           |
| `StepFixture`                                      | Build a minimal `auditlog.StepInfo` for visualization/table tests                                          |
| `RetryOpts`                                        | Build retry config with a fresh backoff instance                                                           |
| `AddRetryStep`                                     | Wrap a step with retry config (fresh backoff)                                                              |
| `AddSingleStep`                                    | Wire a single succeed step into a workflow                                                                 |
| `RunSingleSucceed`                                 | Run minimal single-succeed-step workflow (auditor + wf + step + Attach + Do + Snapshot)                    |
| `RunSingleSucceedWithBuffer`                       | `RunSingleSucceed` + fresh `*strings.Builder` for `Write*`-into-buffer tests                               |
| `RunSingleSucceedWithReport`                       | `RunSingleSucceed().Report()` — returns the assembled `WorkflowReport` for tests that only need the report |
| `RunWorkflow`                                      | `Attach` + `Do` + `Snapshot` in one call; accepts `testing.TB` so benchmarks can reuse the same setup      |
| `SingleSucceedExportPath`                          | `RunSingleSucceed` + `t.TempDir`-anchored path for `Export*` tests                                         |
| `FindStep`, `AssertReportValid`                    | Step lookup + structural validation                                                                        |
| `AssertStepCount`                                  | Required step count (uses `Fatalf` to stop on mismatch)                                                    |
| `AssertEventCount`                                 | Required event count (`Errorf` — multiple counts may co-fail)                                              |
| `AssertCount(name, got, want)`                     | Generic named-count assertion                                                                              |
| `AssertWorkflowID`                                 | Required WorkflowID                                                                                        |
| `AssertAttemptCount`                               | Required attempt count for a StepInfo                                                                      |
| `AssertStatus`                                     | Required status for a StepInfo                                                                             |
| `AssertFirstStepName`                              | Required name of `report.Steps[0]`                                                                         |
| `AssertContains`                                   | `strings.Contains` check with custom failure message                                                       |
| `FailingWriter`, `ErrWriteFailed`                  | Shared `io.Writer` that always fails — used by error-path tests in both core and viz                       |

### Duplicate-code policy

- Run `art-dupl --semantic --sort total-tokens -t 15` to find clones.
- Goal is **zero harmful duplication**, not zero report lines. Some
  signature-only matches (e.g. multiple `Assert*(t, report, want)` helpers
  sharing the same parameter shape) are intentional: each helper asserts a
  different field with different semantics and merging would harm clarity.
- Production-code duplication is never acceptable: extract helpers (see
  `sortByName`, `sortStepsByName`, `diffStep`, `writeGraph` in `viz/render.go`).
- **Current state**: zero clone groups at any threshold from `-t 3` through
  `-t 30` (production code extracted via `writeGraph` in `viz/render.go`; test
  `Write*`-into-buffer preamble extracted via
  `RunSingleSucceedWithBuffer` in `testhelpers`, eliminating the
  23-occurrence `t.Parallel + runSingleSucceed + var buf strings.Builder`
  clone group that previously appeared at `-t 3`; test `Export*` preamble
  extracted via `SingleSucceedExportPath` in `testhelpers`). The
  formerly-documented "ten acceptable clones" section below was retired
  when the refactor landed — those patterns no longer appear in the
  report.

#### Acceptable clones (documented in source)

**Zero clone groups at every threshold from `-t 3` through `-t 30`.** The
final 10 occurrences of the `t.Parallel() + RunSingleSucceed(t, name) +
report := a.Report()` preamble were eliminated by adding
`RunSingleSucceedWithReport(t, name) *auditlog.WorkflowReport` to
`testhelpers`. The 2-occurrence benchmark `a.Attach(w) / w.Do / a.Snapshot(w)`
group was eliminated by generalizing `RunWorkflow` to accept
`testing.TB` (so benchmarks reuse the same setup).

#### Helper additions

- **`AddLinearChain(w, a, b, c)`** (`testhelpers`): wires a 3-step
  linear dependency chain (`a → b → c`) into a workflow. Centralizes
  the `w.Add(flow.Step(a), flow.Step(b).DependsOn(a),
flow.Step(c).DependsOn(b))` idiom previously duplicated across
  `diagram_test.go` and `html_test.go`. Companion to the existing
  `AddDependentStep` (2-step chain) and `AddParallelSteps` (no edges).

- **`SingleSucceedExportPath(t, stepName, fileName)`** (`testhelpers`):
  runs a single-succeed workflow with the given step name and returns
  the auditor plus a `t.TempDir()`-anchored output path. Centralizes the
  `runSingleSucceed + t.TempDir + path` boilerplate shared by every
  Export\* test (Mermaid, PlantUML, Graphviz, D2, JSON, HTML, table,
  tree, HTML tree — 10 call sites). Callers still invoke `t.Parallel()`
  at the test level so the paralleltest linter stays satisfied.

- **`RunSingleSucceedWithBuffer(t, name)`** (`testhelpers`): runs a
  single-succeed workflow and returns the auditor plus a fresh
  `*strings.Builder` ready to receive `Write*` (non-String) output.
  Centralizes the `t.Parallel + RunSingleSucceed + var buf strings.Builder`
  preamble previously duplicated across 23 sites in
  `diagram_direction_test.go` (18), `output_test.go` (3),
  `table_columns_test.go` (1), and `html_test.go` (1). Tests that only
  call `Write*String` variants may discard the buffer with `_`.
  Callers still invoke `t.Parallel()` at the test level.

- **`RunSingleSucceedWithReport(t, name) auditlog.WorkflowReport`**
  (`testhelpers`): runs a single-succeed workflow and returns the
  assembled `WorkflowReport` directly. Centralizes the
  `t.Parallel + a := RunSingleSucceed + report := a.Report()` preamble
  across 10 sites that only need the report (no auditor queries).
  Reduces the callsite to `t.Parallel()` + 1 helper call. Use
  `RunSingleSucceed` instead when the test needs the auditor handle.

- **`RunWorkflow(tb testing.TB, ...)`** (`testhelpers`): generalized
  from `*testing.T` to `testing.TB` so benchmarks (`*testing.B`) can
  reuse the same `Attach + Do + Snapshot` setup. Replaces the
  duplicate `a.Attach(w) / w.Do / a.Snapshot(w)` sequence in
  `viz/benchmarks_test.go`.
