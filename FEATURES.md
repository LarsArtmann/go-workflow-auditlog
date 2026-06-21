# Features — go-workflow-auditlog

Honest feature inventory by status. Verified against the codebase on 2026-06-21.

**Module**: `github.com/larsartmann/go-workflow-auditlog` · **Go**: 1.26+ · **Status**: ALPHA

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
- **`Validate()`** — checks count consistency (event, step, 6 status-count fields) + status drift via sentinel errors
- **`Filtered(opts...)`** — filter by step name, status, event type, time range
- **`Diff(other)`** — compare two runs (added/removed/changed steps + wall-clock duration delta)
- **`Summary()`** — one-line human-readable summary (uses wall-clock + failure reason)
- **`Duration()`** — wall-clock duration as `time.Duration`
- **`ReportIndex`** — O(1) lookup maps for repeated queries
- **`ReplayEvents()`** — reconstruct report from flat NDJSON event stream
- **`LoadReport()` / `LoadReportFromReader()` / `LoadReportFromBytes()`**
- **`ReadEvents()`** — NDJSON reader (inverse of WriteNDJSON)

### Report Aggregate Fields

- `WallClockDurationMs` — actual elapsed time (earliest → latest event)
- `PeakConcurrency` — max in-flight attempts (event-stream scan)
- `CriticalPathDurationMs` — longest dependency-chain duration (memoized DFS)
- `FailureReason` — human-readable failure summary
- `PendingCount` / `RunningCount` — split lifecycle-state counters
- `TotalDurationMs` — sum of per-step durations (kept for completeness)

### Export Formats

| Format                 | Write (writer)  | WriteString           | Export (file)    | On Auditor | On Report |
| ---------------------- | --------------- | --------------------- | ---------------- | ---------- | --------- |
| JSON report            | `WriteJSON`     | —                     | `ExportJSON`     | ✅         | ✅        |
| NDJSON events          | `WriteNDJSON`   | —                     | `ExportNDJSON`   | ✅         | ✅        |
| Mermaid                | `WriteMermaid`  | `WriteMermaidString`  | `ExportMermaid`  | ✅         | ✅        |
| PlantUML               | `WritePlantUML` | `WritePlantUMLString` | `ExportPlantUML  | ✅         | ✅        |
| Graphviz DOT           | `WriteGraphviz` | `WriteGraphvizString` | `ExportGraphviz` | ✅         | ✅        |
| D2                     | `WriteD2`       | `WriteD2String`       | `ExportD2`       | ✅         | ✅        |
| Table (16 sub-formats) | `WriteTable`    | `WriteTableString`    | `ExportTable`    | ✅         | ✅        |
| ASCII Tree             | `WriteTree`     | `WriteTreeString`     | `ExportTree`     | ✅         | ✅        |
| HTML Tree              | `WriteHTMLTree` | `WriteHTMLTreeString` | `ExportHTMLTree` | ✅         | ✅        |

Table sub-formats: table, json, csv, tsv, markdown, xml, d2, yaml, html, tree, mermaid, dot, jsonl, asciidoc, toml, plantuml

### Diagram Quality

- **Single source of truth for colors**: `StepStatus.Color()` — all renderers delegate
- **Single DAG builder**: `buildGraph()` feeds Mermaid, Graphviz, PlantUML, D2
- **Single label function**: `stepLabel()` used by all renderers
- **Edges follow execution flow** (dependency → step) — matches tree, matches GitHub Actions / Airflow convention
- **D2 title derived from `WorkflowID`** (self-labeling, not hardcoded)

### API Symmetry

- **Full `Write*` / `Write*String` / `Export*` on both `Auditor` and `WorkflowReport`**
- **Canonical JSON/NDJSON names** (`WriteJSON`, `WriteNDJSON`, `ExportJSON`, `ExportNDJSON`) with deprecated backward-compat aliases (`WriteReportJSON`, `ExportToFile`, etc.)

### Infrastructure

- **go-output** dependency at v0.17.0 (root + all sub-modules aligned)
- **golangci-lint v2** with depguard allow-list, 0 issues
- **`.goreleaser.yml`** for automated GitHub releases

### Documentation

- `AGENTS.md` — comprehensive session context (file map, data flow, gotchas, testing patterns)
- `README.md` — end-user guide with API reference, examples, 3-duration-metrics explainer
- `CHANGELOG.md` — [Unreleased] populated
- `docs/DOMAIN_LANGUAGE.md` — DDD glossary
- `example/main.go` — demos all export formats via `--export` flag

---

## PARTIALLY DONE ⚠️

### Table Column Configuration

`buildTableData()` hardcodes 5 columns: Step, Status, Duration, Attempts, Error. No way for users to customize which columns appear or add custom columns. The `HasRetry`, `HasTimeout`, and `StepType` fields exist on `StepInfo` but are not available as table columns.

### Diagram Layout Direction

No way to set Mermaid/D2/Graphviz layout direction (TD vs LR). All diagrams default to top-down.

---

## PLANNED (see TODO_LIST.md and ROADMAP.md)

- `flake.nix` migration (replace deprecated justfile)
- Make heavy deps optional (split into core + visualization sub-modules, or build tags)
- Streaming NDJSON export (write events as captured, not buffered)
- OpenTelemetry span bridge
- `encoding/json/v2` migration (Go 1.25+ policy)
- HTML dashboard report (self-contained, combines table + diagram + tree)
- Branded `RunID` string type (stronger types)
- `Name(step)` fallback helper (deterministic step names when `String()` is unset)
- Fuzz tests for diagram ID sanitization
- Benchmarks for new render paths (WriteD2, WriteTable, WriteTree, analytics)
