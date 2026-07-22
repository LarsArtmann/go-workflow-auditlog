# Features — go-workflow-auditlog

Honest feature inventory by status. Verified against the codebase on 2026-07-13.

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

| Format                 | Write (writer)  | WriteString           | Export (file)    | On Auditor | On Report |
| ---------------------- | --------------- | --------------------- | ---------------- | ---------- | --------- |
| JSON report            | `WriteJSON`     | —                     | `ExportJSON`     | ✅         | ✅        |
| NDJSON events          | `WriteNDJSON`   | —                     | `ExportNDJSON`   | ✅         | ✅        |
| Mermaid                | `WriteMermaid`  | `WriteMermaidString`  | `ExportMermaid`  | ✅         | ✅        |
| PlantUML               | `WritePlantUML` | `WritePlantUMLString` | `ExportPlantUML` | ✅         | ✅        |
| Graphviz DOT           | `WriteGraphviz` | `WriteGraphvizString` | `ExportGraphviz` | ✅         | ✅        |
| D2                     | `WriteD2`       | `WriteD2String`       | `ExportD2`       | ✅         | ✅        |
| Table (16 sub-formats) | `WriteTable`    | `WriteTableString`    | `ExportTable`    | ✅         | ✅        |
| ASCII Tree             | `WriteTree`     | `WriteTreeString`     | `ExportTree`     | ✅         | ✅        |
| HTML Tree              | `WriteHTMLTree` | `WriteHTMLTreeString` | `ExportHTMLTree` | ✅         | ✅        |
| HTML Dashboard         | `WriteHTML`     | `WriteHTMLString`     | `ExportHTML`     | ✅         | ✅        |

Table sub-formats: table, json, csv, tsv, markdown, xml, d2, yaml, html, tree, mermaid, dot, jsonl, asciidoc, toml, plantuml

### Diagram Quality

- **Single source of truth for colors**: `StepStatus.Color()` — all renderers delegate
- **Single DAG builder**: `buildGraph()` feeds Mermaid, Graphviz, PlantUML, D2
- **Single label function**: `stepLabel()` used by all renderers
- **Edges follow execution flow** (dependency → step) — matches tree, matches GitHub Actions / Airflow convention
- **D2 title derived from `WorkflowID`** (self-labeling, not hardcoded)

### API Symmetry

- **Full `Write*` / `Write*String` / `Export*` on both `Auditor` and `WorkflowReport`**
- **Canonical JSON/NDJSON names** (`WriteJSON`, `WriteNDJSON`, `ExportJSON`, `ExportNDJSON`)
- **Backward-compatible variadic options** — all diagram writers (`WriteMermaid`, `WriteGraphviz`, `WriteD2`, `WritePlantUML`) and table writers (`WriteTable`) accept optional `...DiagramOption` / `...TableOption` without breaking existing callers

### Configurable Output Options

- **Configurable table columns** — `WithColumns(TableColumn...)` selects which columns appear in table export. 10 columns available: Step, Status, Duration, Attempts, MaxAttempts, Retry, Timeout, Error, Type, Dependencies. Default preserves backward compatibility (original 7). Works across all 16 table sub-formats.
- **Diagram layout direction** — `WithDirection(output.Direction)` sets TD/LR/BT/RL on Mermaid, Graphviz, D2, and PlantUML. Uses native go-output renderer support for DOT and D2; post-processing for Mermaid and PlantUML.

### Infrastructure

- **go-output** dependency at v0.30.4 (root + all sub-modules aligned)
- **go-error-family** at v0.7.0
- **golangci-lint v2** with depguard allow-list, pinned to v2.12.2 in CI
- **govulncheck** in CI (golang/govulncheck-action)
- **actionlint** in CI (workflow linting)
- **Coverage gate** at 92%
- **flake.nix** devShell (Go 1.26.4, golangci-lint, govulncheck, actionlint; GOEXPERIMENT=jsonv2)
- **flake-parts** + **treefmt-nix** for build automation
- **Pre-commit hook** (vet + lint + test)
- **STABILITY.md** documenting API stability promises
- **`.goreleaser.yml`** for automated GitHub releases

### Documentation

- `AGENTS.md` — comprehensive session context (file map, data flow, gotchas, testing patterns)
- `README.md` — end-user guide with API reference, examples, 3-duration-metrics explainer
- `CHANGELOG.md` — v0.7.0 tagged (table columns + diagram direction + CriticalPath/PeakConcurrencySteps + json/v2 + website)
- `docs/DOMAIN_LANGUAGE.md` — DDD glossary
- `example/main.go` — demos all export formats via `--export` flag

### Testing & Quality

- **`encoding/json/v2` migration** — migrated to Go 1.26 `encoding/json/v2` + `jsontext` (GOEXPERIMENT=jsonv2), full XSS hardening, deterministic output
- **Fuzz tests**: `FuzzDiagramSpecialChars` (diagram injection), `FuzzDiagramSanitization_MultiStep` (multi-step edge sanitization, 17 seed pairs across 4 formats), `FuzzHTMLSpecialChars` (HTML XSS, 12 seed payloads), `FuzzReadEvents` (NDJSON resilience), `FuzzClassify` (adversarial error chains)
- **Property-based tests**: Diff algebra (identity, added/removed duality, duration anti-symmetry, status-change symmetry, sorted output) — 200 iterations each, deterministic seeds; Classify wrapping-preserves-family + identity matches map
- **Atomic file writes**: crash-safe export (temp file + rename + bufio)
- **Enum validation on ingest**: ReadEvents rejects unknown event_type/phase values
- **Benchmarks**: runtime overhead (Invocation, Attach, BuildReport, EventsCopy, OnEventCallback, RetryWithAudit) + export rendering (WriteD2/Table/Tree/JSON/Mermaid on 100-step reports) + renderHTML (small 3-step + large 1000-step) + godoc examples

---

## PLANNED (see TODO_LIST.md and ROADMAP.md)

- Make heavy deps optional (split into core + visualization sub-modules, or build tags)
- Streaming NDJSON export (write events as captured, not buffered)
- OpenTelemetry span bridge
- CLI tool (`auditlog`) for inspecting/replaying/diffing exported reports
