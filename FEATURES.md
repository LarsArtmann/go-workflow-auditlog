# Features — go-workflow-auditlog

Honest feature inventory by status. Verified against the codebase on 2026-07-24.

**Modules**: `github.com/larsartmann/go-workflow-auditlog` (core) · `…/viz` (visualization) · `…/live` (real-time dashboard) · **Go**: 1.26+ · **Status**: ALPHA

---

## DONE ✅

### Core Audit Pipeline

- **Auditor lifecycle**: `New(Config)` → `Attach(w)` → `w.Do(ctx)` → `Snapshot(w)` → `Report()`
- **Per-attempt event capture**: `attempt_start` / `attempt_end` with timestamps, errors, durations
- **Full step DAG capture**: dependencies, dependents, retry/timeout config, step types
- **Sub-workflow traversal** via `flow.Traverse` (captures inner steps that bypass callbacks)
- **Skipped & canceled detection** (reads post-execution state for steps that bypass Before/After)
- **`StepInfo.Error` reflects FINAL outcome only** — a succeeded step has `Error == nil` even after transient failures (regression-tested)
- **MaxEvents cap** with `DroppedEventCount` tracking

### Report & Query API

- **`WorkflowReport`** with denormalized aggregates (counts, durations, peak concurrency, critical path)
- **Branded `RunID` type** (`type RunID string`) — compile-time safety against confusing `RunID` with `WorkflowID`, serializes as a plain JSON string
- **`Validate()`** — checks count consistency (event, step, 6 status-count fields) + status drift via sentinel errors
- **`Filtered(opts...)`** — filter by step name, status, event type, time range
- **`Diff(other)`** — compare two runs (added/removed/changed steps + wall-clock duration delta)
- **`Summary()`** — one-line human-readable summary (uses wall-clock + failure reason)
- **`Duration()`** — wall-clock duration as `time.Duration`
- **`CriticalPath()`** — returns the ordered step chain (root-to-leaf) of the bottleneck dependency path
- **`PeakConcurrencySteps()`** — returns the unique steps that were in-flight at peak concurrency
- **`ReportIndex`** — O(1) lookup maps for repeated queries
- **`ReplayEvents()`** — reconstruct report from flat NDJSON event stream
- **`LoadReport()` / `LoadReportFromReader()` / `LoadReportFromBytes()`**
- **`ReadEvents()`** — NDJSON reader (inverse of WriteNDJSON)
- **Streaming NDJSON** (`NDJSONStreamer` in `stream.go`) — real-time event streaming via `Config.OnEvent`; thread-safe mutex-protected writes, 64KB default buffer (configurable via `WithBufferSize`), `WithAutoFlush()` for tailing, `CreateNDJSONStreamer(path)` file convenience constructor, first-error-wins error handling. Output is `ReadEvents`-compatible.

### Error Classification

- **[go-error-family](https://github.com/larsartmann/go-error-family) integration** — all 12 public sentinel errors auto-registered with behavioral `Family` classification on import via `init()` into `DefaultRegistry`
- **12 public sentinels** classified: 4 Corruption (exit 65), 5 Rejection (exit 1), 1 Transient (exit 75, retryable), 2 Infrastructure (exit 69)
- **`Classify(err)`**, **`IsRetryable(err)`**, **`ExitCode(err)`** work on any auditlog error — no consumer-side setup needed
- **`errors.Is` semantics unchanged** — registration is additive metadata, not replacement (Strategy A)
- **`RegisterClassifications(reg)`** for custom registries; **`ErrorClassifications()`** returns the canonical mapping
- **All I/O error paths wrapped** — render, write, load, flush, rename failures carry matchable sentinels

### Report Aggregate Fields

- `WallClockDurationMs` — actual elapsed time (earliest → latest event)
- `PeakConcurrency` — max in-flight attempts (event-stream scan)
- `CriticalPathDurationMs` — longest dependency-chain duration (memoized DFS)
- `FailureReason` — human-readable failure summary
- `PendingCount` / `RunningCount` — split lifecycle-state counters
- `TotalDurationMs` — sum of per-step durations (kept for completeness)

### Export Formats

**Core module** (JSON/NDJSON — methods on `Auditor` and `WorkflowReport`):

| Format        | Write (writer)  | Export (file)  | On Auditor | On Report |
| ------------- | --------------- | -------------- | ---------- | --------- |
| JSON report   | `WriteJSON`     | `ExportJSON`   | ✅         | ✅        |
| NDJSON events | `WriteNDJSON`   | `ExportNDJSON` | ✅         | ✅        |

**Visualization module** (diagrams/tables/trees/HTML — package-level functions in `viz`):

| Format                 | Write (writer)           | WriteString           | Export (file)          |
| ---------------------- | ------------------------ | --------------------- | ---------------------- |
| Mermaid                | `viz.WriteMermaid`       | `viz.WriteMermaidString`  | `viz.ExportMermaid`       |
| PlantUML               | `viz.WritePlantUML`      | `viz.WritePlantUMLString` | `viz.ExportPlantUML`      |
| Graphviz DOT           | `viz.WriteGraphviz`      | `viz.WriteGraphvizString` | `viz.ExportGraphviz`      |
| D2                     | `viz.WriteD2`            | `viz.WriteD2String`       | `viz.ExportD2`            |
| Table (16 sub-formats) | `viz.WriteTable`         | `viz.WriteTableString`    | `viz.ExportTable`         |
| ASCII Tree             | `viz.WriteTree`          | `viz.WriteTreeString`     | `viz.ExportTree`          |
| HTML Tree              | `viz.WriteHTMLTree`      | `viz.WriteHTMLTreeString` | `viz.ExportHTMLTree`      |
| HTML Dashboard         | `viz.WriteHTML`          | `viz.WriteHTMLString`     | `viz.ExportHTML`          |

**Live module** (real-time streaming):

| Format          | Mechanism                          | Constructor                    |
| --------------- | ---------------------------------- | ------------------------------ |
| Streaming NDJSON | `NDJSONStreamer.OnEvent`          | `NewNDJSONStreamer` / `CreateNDJSONStreamer` |
| Live SSE events  | `hub.OnEvent` → SSE fan-out       | `live.New(config, serverConfig)` |

Table sub-formats: table, json, csv, tsv, markdown, xml, d2, yaml, html, tree, mermaid, dot, jsonl, asciidoc, toml, plantuml

**Streaming NDJSON** (core module) writes events in real time via `Config.OnEvent` — no need to wait for `Report()`. Thread-safe, 64 KB buffer (configurable), auto-flush option. Output is `ReadEvents`-compatible.

**Live SSE** (live module) streams events to browser dashboards in real time. Steps light up as they execute; the DAG graph activates on `SignalComplete()`.

### Diagram Quality

- **Single source of truth for colors**: `StepStatus.Color()` — all renderers delegate
- **Single DAG builder**: `buildGraph()` feeds Mermaid, Graphviz, PlantUML, D2
- **Single label function**: `stepLabel()` used by all renderers
- **Edges follow execution flow** (dependency → step) — matches tree, matches GitHub Actions / Airflow convention
- **D2 title derived from `WorkflowID`** (self-labeling, not hardcoded)

### API Symmetry

- **Core JSON/NDJSON** on both `Auditor` and `WorkflowReport` (`WriteJSON`, `WriteNDJSON`, `ExportJSON`, `ExportNDJSON`)
- **Viz diagrams/tables/trees/HTML** as package-level functions taking `WorkflowReport` as first arg (`viz.WriteMermaid(report, w)`, `viz.ExportHTML(report, path)`, etc.)
- **Canonical JSON/NDJSON names** (`WriteJSON`, `WriteNDJSON`, `ExportJSON`, `ExportNDJSON`)
- **Backward-compatible variadic options** — all diagram writers (`viz.WriteMermaid`, `viz.WriteGraphviz`, `viz.WriteD2`, `viz.WritePlantUML`) and table writers (`viz.WriteTable`) accept optional `...DiagramOption` / `...TableOption` without breaking existing callers

### Configurable Output Options

- **Configurable table columns** — `WithColumns(TableColumn...)` selects which columns appear in table export. 10 columns available: Step, Status, Duration, Attempts, MaxAttempts, Retry, Timeout, Error, Type, Dependencies. Default preserves backward compatibility (original 7). Works across all 16 table sub-formats.
- **Diagram layout direction** — `WithDirection(output.Direction)` sets TD/LR/BT/RL on Mermaid, Graphviz, D2, and PlantUML. Uses native go-output renderer support for DOT and D2; post-processing for Mermaid and PlantUML.

### Dashboard Visualization Enhancements

- **Critical path highlighting (graph)** — toggle button highlights the longest-duration dependency chain with glowing accent strokes on nodes and thicker accent edges; critical-path steps injected from Go (`CriticalPathSteps` field) with client-side JS fallback
- **Critical path overlay (Gantt)** — timeline bars on the critical path get accent-colored glow and bold labels
- **Duration labels on graph nodes** — compact inline duration (e.g., `fetch · 10ms`) via `humanizeMs()` helper in `daghtml_adapter.go`
- **Retry count badges** — `↻N` amber badge on graph nodes with `attempt_count > 1`
- **Graph search/filter** — search input highlights matching nodes (info stroke) and dims non-matches to 15% opacity
- **Idempotent graph enhancement** — `enhanceGraph()` guards against duplicate badge/listener application on repeated tab switches

### Live Real-Time Dashboard (`live/` module)

- **SSE streaming dashboard** — real-time HTTP dashboard where steps light up as they execute, with incremental rendering via `requestAnimationFrame` batching
- **Hub** — SSE subscriber registry with non-blocking fan-out `OnEvent` broadcast; `SignalComplete()` notifies all clients when the workflow finishes
- **Server** — HTTP server with SSE handler (`/api/events`), `/api/report`, `/api/health`, dashboard serving, `ServeHTTP` for `http.Handler` integration
- **`live.New(config, serverConfig)`** — convenience constructor that wires `hub.OnEvent` as `Config.OnEvent`, returns `(*Server, *Auditor, error)`
- **Live data flow** — browser connects to `/api/events` → receives `snapshot` event (current report + events + metadata + DAG) → incremental `event` messages as steps execute → `complete` event with final report + full DAG on `SignalComplete()`
- **Demo pipeline** at `live/demo` (fetch → validate → transform → save → notify with retry) serving at `http://localhost:18080`
- **Depends on `go-sse`** (`github.com/larsartmann/go-sse`, private, `replace` directive; will be removed once go-sse is public)

### Infrastructure

- **Three Go modules**: core (`auditlog`), visualization (`viz`), live dashboard (`live`) — linked via `go.work` workspace
- **go-output** dependency at v0.31.1 in `viz/go.mod` (includes D2/DOT quoting fix)
- **go-error-family** at v0.7.0
- **go-sse** at v0.0.0 (private, `replace` directive in `live/go.mod`)
- **golangci-lint v2** with depguard allow-list, pinned to v2.12.2 in CI
- **govulncheck** in CI (golang/govulncheck-action)
- **actionlint** in CI (workflow linting)
- **Coverage**: core 95.5%, viz 91.7%, live 76.9% (CI gate ≥92% for core + viz combined)
- **flake.nix** devShell (Go 1.26.4, golangci-lint, govulncheck, actionlint, `d2` CLI; GOEXPERIMENT=jsonv2)
- **flake-parts** + **treefmt-nix** for build automation (includes `d2-fmt`, `nixfmt`, `gofmt`)
- **Pre-commit hook** (vet + lint + test)
- **STABILITY.md** documenting API stability promises
- **`.goreleaser.yml`** for automated GitHub releases

### Documentation

- `AGENTS.md` — comprehensive session context (file map, data flow, gotchas, testing patterns, 3-module architecture)
- `README.md` — end-user guide with API reference, examples, 3-duration-metrics explainer, streaming section, screenshots
- `CHANGELOG.md` — v0.7.0 tagged (table columns + diagram direction + CriticalPath/PeakConcurrencySteps + json/v2 + website); `[Unreleased]` covers streaming NDJSON, DAG viz enhancements, module split, live dashboard module
- `docs/DOMAIN_LANGUAGE.md` — DDD glossary
- `example/main.go` — demos all export formats via `--export` flag (in `viz/` module)
- `live/demo/main.go` — demos real-time SSE dashboard with retry pipeline

### Testing & Quality

- **`encoding/json/v2` migration** — migrated to Go 1.26 `encoding/json/v2` + `jsontext` (GOEXPERIMENT=jsonv2), full XSS hardening, deterministic output
- **Fuzz tests**: `FuzzDiagramSpecialChars` (diagram injection), `FuzzDiagramSanitization_MultiStep` (multi-step edge sanitization, 17 seed pairs across 4 formats), `FuzzHTMLSpecialChars` (HTML XSS, 12 seed payloads), `FuzzReadEvents` (NDJSON resilience), `FuzzClassify` (adversarial error chains)
- **Property-based tests**: Diff algebra (identity, added/removed duality, duration anti-symmetry, status-change symmetry, sorted output) — 200 iterations each, deterministic seeds; Classify wrapping-preserves-family + identity matches map
- **Atomic file writes**: crash-safe export (temp file + rename + bufio)
- **Enum validation on ingest**: ReadEvents rejects unknown event_type/phase values
- **Benchmarks**: runtime overhead (Invocation, Attach, BuildReport, EventsCopy, OnEventCallback, RetryWithAudit) + export rendering (WriteD2/Table/Tree/JSON/Mermaid on 100-step reports) + renderHTML (small 3-step + large 1000-step) + NDJSONStreamer throughput (100/1000/10000 events) + godoc examples
- **~389 test functions** across 3 modules (core: 135 tests + 5 examples + 3 benchmarks + 4 fuzz; viz: 193 tests + 16 examples + 15 benchmarks + 3 fuzz; live: 15 tests)

---

## PLANNED (see TODO_LIST.md and ROADMAP.md)

- OpenTelemetry span bridge
- CLI tool (`auditlog`) for inspecting/replaying/diffing exported reports
- Live DAG graph during execution (needs DAG structure available before `Do()`; currently only renders on `snapshot`/`complete` events)
- Dashboard step-table diff-based DOM updates (eliminate flicker for 100+ steps)
- Minimap for large graphs (>20 nodes)
- CONTRIBUTING.md
