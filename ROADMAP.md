# Roadmap — go-workflow-auditlog

Long-term direction and raw ideas not yet refined into actionable tasks.
Short- and mid-term work lives in [TODO_LIST.md](./TODO_LIST.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md).

**Module**: `github.com/larsartmann/go-workflow-auditlog` · **Status**: ALPHA

---

## Direction

The library's core mission is **trustworthy, observable audit trails for
[go-workflow](https://github.com/Azure/go-workflow) pipelines**. Every step
execution event — attempts, retries, durations, errors, dependencies, final
statuses — captured with timestamps and exportable to any format.

The long-term arc moves from "capture and export" toward **analyze and act**:

1. **Capture** (done) — per-attempt events, DAG structure, sub-workflow traversal
2. **Export** (done) — JSON, NDJSON, Mermaid, PlantUML, DOT, D2, 16 table formats, ASCII/HTML trees, interactive HTML dashboard
3. **Analyze** (in progress) — wall-clock vs total vs critical-path metrics, diff/regression detection, peak concurrency
4. **Act** (future) — OpenTelemetry bridge, streaming export, replay UI, alerting

---

## Themes

### Dependency Architecture

**Done (2026-07-22).** The library is split into two independent Go modules:

- **Core** (`github.com/larsartmann/go-workflow-auditlog`) — event capture,
  JSON/NDJSON export, replay, diff, filter. 3 direct deps, no go-output.
- **Visualization** (`github.com/larsartmann/go-workflow-auditlog/viz`) —
  diagrams, tables, trees, HTML dashboard. Depends on core + go-output.

Consumers who only need JSON/NDJSON audit trails import the core module and pay
zero go-output dependency cost. Both modules share a `go.work` workspace in
development; CI also verifies `GOWORK=off` standalone builds.

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

---

## Raw Ideas (not yet scoped)

- CLI tool (`auditlog`) for inspecting/replaying/diffing exported reports
- `FailureReason` structured categories (typed categories, not just a string)
- `Diff()` on PeakConcurrency / CriticalPath (currently only duration)
- `ReplayEvents` round-trip property/fuzz test (export → read → replay = equivalent)
- Configurable node shapes/icons per step type in diagrams
- Workflow-level retry/timeout surfacing in the report

---

## Strategic First-Chunk Audits

### Module Split (P6-38) — Done

**Shipped (2026-07-22).** Core module (`auditlog` package, 3 deps) and
visualization module (`viz` package, go-output deps) are separate Go modules
linked via `go.work`. Core consumers pay zero go-output dependency cost.

### Streaming NDJSON Export (P6-39) — API Design

```go
func (a *Auditor) StreamEvents(w io.Writer) func() error
```

Writes events as NDJSON as captured via `OnEvent`. **Defer** until real-time
tailing need arises — current buffer handles 10k+ events fine.

### OpenTelemetry Span Bridge (P6-40) — Mapping

`attempt_start` → span start, `attempt_end` → span end with attributes.
**Defer** until a consumer has an OTel stack.

### init() Auto-Registration Decision (P6-36) — Recommendation

**KEEP the `init()` auto-registration.** Follows standard Go pattern
(database/sql, image codecs). Zero setup for consumers. Escape hatch exists
via `RegisterClassifications(reg)`. No consumer has reported issues.
