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
2. **Export** (done) — JSON, NDJSON (batch + real-time streaming), Mermaid, PlantUML, DOT, D2, 16 table formats, ASCII/HTML trees, interactive HTML dashboard
3. **Analyze** (done) — wall-clock vs total vs critical-path metrics, diff/regression detection, peak concurrency, critical-path step chain
4. **Act** (future) — OpenTelemetry bridge, alerting, replay UI

---

## Themes

### Dependency Architecture

**Done (2026-07-22).** The library is split into two independent Go modules:

- **Core** (`github.com/larsartmann/go-workflow-auditlog`) — event capture,
  JSON/NDJSON export (batch + streaming), replay, diff, filter, index,
  error classification. 3 direct deps, no go-output.
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

**Streaming NDJSON is shipped (2026-07-22).** `NDJSONStreamer` in `stream.go`
writes events as NDJSON in real time via the `Config.OnEvent` callback — no
full in-memory buffer required. Thread-safe (mutex-protected writes),
64 KB default buffer (configurable via `WithBufferSize`), `WithAutoFlush()`
for immediate visibility, `CreateNDJSONStreamer(path)` file convenience
constructor. Output is `ReadEvents`-compatible for round-trip replay.

**Remaining scale direction**: The in-memory `Report()` path still
materializes all events. For workflows exceeding 10000+ events, a
streaming-only consumption model (skip `Report()` entirely) is the path
forward. Potential enhancements: time-based flush (`WithFlushInterval`),
async channel-based writer for backpressure decoupling, and a streaming
JSON report format (not just NDJSON events).

### Observability Integration

**Direction**: Add an **OpenTelemetry span bridge** that maps `attempt_end`
events to OTel spans. Consumers get distributed tracing for free — every
workflow step becomes a span with the audit log's rich metadata (retry count,
durations, dependency chain). This is the "act" layer: the audit log feeds
existing observability stacks rather than requiring a separate dashboard.

**Defer** until a consumer has an OTel stack.

---

## Raw Ideas (not yet scoped)

- CLI tool (`auditlog`) for inspecting/replaying/diffing exported reports
- `FailureReason` structured categories (typed enum, not just a string)
- `Diff()` on PeakConcurrency / CriticalPath (currently only duration delta)
- Configurable node shapes/icons per step type in diagrams
- Workflow-level retry/timeout surfacing in the report
- Time-based streaming flush (`WithFlushInterval(d time.Duration)`)
- Async channel-based streaming writer (decouple step execution from I/O latency)
- `MultiWriter` that fans events to multiple `OnEvent` callbacks simultaneously

---

## Strategic Decisions

### Module Split — Done

**Shipped (2026-07-22).** Core module (`auditlog` package, 3 deps) and
visualization module (`viz` package, go-output deps) are separate Go modules
linked via `go.work`. Core consumers pay zero go-output dependency cost.

### Streaming NDJSON Export — Done

**Shipped (2026-07-22).** `NDJSONStreamer` provides real-time event streaming
via `Config.OnEvent`. Actual API:

```go
streamer := auditlog.NewNDJSONStreamer(w, auditlog.WithAutoFlush())
auditor := auditlog.New(auditlog.Config{OnEvent: streamer.OnEvent})
// ... Attach, Do, Snapshot ...
streamer.Close()
```

File convenience: `auditlog.CreateNDJSONStreamer(path, opts...)`.

### init() Auto-Registration — Decision: KEEP

**KEEP the `init()` auto-registration** of error classifications. Follows
standard Go pattern (database/sql, image codecs). Zero setup for consumers.
Escape hatch exists via `RegisterClassifications(reg)`. No consumer has
reported issues.

### OpenTelemetry Span Bridge — Deferred

`attempt_start` → span start, `attempt_end` → span end with attributes.
**Defer** until a consumer has an OTel stack.
