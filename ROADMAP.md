# Roadmap — go-workflow-auditlog

Long-term direction and raw ideas not yet refined into actionable tasks.
Short- and mid-term work lives in [TODO_LIST.md](./TODO_LIST.md).

**Module**: `github.com/larsartmann/go-workflow-auditlog` · **Status**: ALPHA

---

## Direction

The library's core mission is **trustworthy, observable audit trails for
[go-workflow](https://github.com/Azure/go-workflow) pipelines**. Every step
execution event — attempts, retries, durations, errors, dependencies, final
statuses — captured with timestamps and exportable to any format.

The long-term arc moves from "capture and export" toward **analyze and act**:

1. **Capture** (done) — per-attempt events, DAG structure, sub-workflow traversal
2. **Export** (done) — JSON, NDJSON, Mermaid, PlantUML, DOT, D2, 16 table formats, ASCII/HTML trees
3. **Analyze** (in progress) — wall-clock vs total vs critical-path metrics, diff/regression detection, peak concurrency
4. **Act** (future) — OpenTelemetry bridge, streaming export, replay UI, alerting

---

## Themes

### Dependency Architecture

The go-output adoption added 12 direct + ~35 transitive dependencies (lipgloss,
yaml, toml). This is fine for a full-featured observability tool but heavy for
consumers who only want JSON/NDJSON event capture.

**Direction**: Consider splitting into a **core module** (JSON/NDJSON only,
2 deps) and a **visualization module** (diagrams/tables/trees, go-output deps).
Consumers opt into the visualization tax. This mirrors how go-output itself is
structured. Decision needed on audience: lightweight audit trail vs full
observability suite.

### Build Automation

The project uses `flake.nix` (flake-parts + treefmt-nix) for build/task
automation. `.goreleaser.yml` handles releases.

**Direction**: Keep `flake.nix` canonical. The pre-commit hook runs all
checks via it; nix makes them reproducible.

### Streaming & Scale

Currently `Report()` materializes all events in memory. For very large
workflows (1000+ steps, 10000+ events) this may be prohibitive.

**Direction**: Add a **streaming NDJSON writer** that writes events as they're
captured (via the `OnEvent` callback) rather than buffering. Pair with a
streaming reader for replay. This enables real-time audit log tailing.

### Observability Integration

**Direction**: Add an **OpenTelemetry span bridge** that maps `attempt_end`
events to OTel spans. Consumers get distributed tracing for free — every
workflow step becomes a span with the audit log's rich metadata (retry count,
durations, dependency chain). This is the "act" layer: the audit log feeds
existing observability stacks rather than requiring a separate dashboard.

### Replay & Visualization

**Done**: The **HTML dashboard report** (`WriteHTML` / `ExportHTML`) is a
self-contained HTML file with 5 tabs (Steps table, DAG Tree, interactive SVG
DAG Graph with Sugiyama layout, Timeline, Events), embedded CSS/JS, and strict
CSP. Consumers get a shareable, offline artifact from any workflow run.

---

## Raw Ideas (not yet scoped)

- CLI tool (`auditlog`) for inspecting/replaying/diffing exported reports
- `encoding/json/v2` migration (Go 1.26+ GOEXPERIMENT=jsonv2) — **DONE** (unreleased): migrated all 5 source files from `encoding/json` to `encoding/json/v2` + `jsontext`. Benefits: ordered maps, ~2x faster, smaller allocations, deterministic output, built-in XSS hardening.
- ~~`go-error-family` adoption for structured, classified errors~~ — **DONE v0.5.0** (Strategy A registration + 3 I/O sentinels + 22 wrapped paths)
- `FailureReason` structured categories (typed categories, not just a string)
- `Diff()` on PeakConcurrency / CriticalPath (currently only duration)
- `ReplayEvents` round-trip property/fuzz test (export → read → replay = equivalent)
- Status report index page (`docs/status/INDEX.md`) linking the 5+ reports
- `CONTRIBUTING.md` documenting the HTML-vs-Markdown snapshot rule
- Configurable node shapes/icons per step type in diagrams
- Workflow-level retry/timeout surfacing in the report

---

## Feature Designs (scoped, awaiting implementation)

### Configurable Table Columns (P4-27) — DONE

`WriteTable` now supports `WithColumns(TableColumn...)` for column selection.
10 columns available: Step, Status, Duration, Attempts, MaxAttempts, Retry,
Timeout, Error, Type, Dependencies. Uses a pre-filter in `buildTableData` —
no upstream go-output changes needed. Backward-compatible variadic option.

### Diagram Layout Direction (P4-28) — DONE

`WithDirection(output.Direction)` sets TD/LR/BT/RL on all diagram formats.
DOT and D2 use native go-output renderer `SetDirection()` support. Mermaid
post-processes `flowchart TD` → `flowchart LR/BT/RL`. PlantUML injects
`left to right direction`. Backward-compatible variadic option.

---

## Strategic First-Chunk Audits

### justfile → flake.nix (P6-37) — DONE

The justfile no longer exists. `flake.nix` is present and BuildFlow runs all
checks via it. This item is complete.

### Module Split (P6-38) — Scope Map

**Current**: single-package library, 55 .go files in root.
**Proposed**: `auditlog-core` (~20 files, 3 deps) + `auditlog-viz` (~11 files,
go-output deps). **Defer** until a consumer requests the lighter footprint.

### Streaming NDJSON Export (P6-39) — API Design

```go
func (a *Auditor) StreamEvents(w io.Writer) func() error
```

Writes events as NDJSON as captured via `OnEvent`. **Defer** until real-time
tailing need arises — current buffer handles 10k+ events fine.

### OpenTelemetry Span Bridge (P6-40) — Mapping

`attempt_start` → span start, `attempt_end` → span end with attributes.
**Defer** until a consumer has an OTel stack.

### encoding/json/v2 Migration (P6-41) — DONE

Migrated to `encoding/json/v2` + `encoding/json/jsontext` via `GOEXPERIMENT=jsonv2`.
All 5 source files (export.go, html_render.go, loader.go, ndjson.go, report.go)
now use the v2 API. Set in `flake.nix` devShell.

### init() Auto-Registration Decision (P6-36) — Recommendation

**KEEP the `init()` auto-registration.** Follows standard Go pattern
(database/sql, image codecs). Zero setup for consumers. Escape hatch exists
via `RegisterClassifications(reg)`. No consumer has reported issues.
