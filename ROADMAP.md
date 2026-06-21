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

The project uses `.goreleaser.yml` + a deprecated `justfile`. The global
AGENTS policy mandates `flake.nix` for build/task automation.

**Direction**: Migrate to `flake.nix` — `nix build`, `nix flake check`,
`nix run .#test`, `nix run .#lint`. The BuildFlow pre-commit hook already runs
the checks; nix makes them canonical and reproducible.

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

**Direction**: An **HTML dashboard report** — a self-contained HTML file that
combines the step table, dependency diagram (Mermaid/D2), and execution tree
into a single navigable page. Like the status reports in `docs/status/` but
generated from a live `WorkflowReport`. Consumers get a shareable, offline
artifact from any workflow run.

### Type Safety

**Direction**: Adopt a **branded `RunID` string type** — `type RunID string`
with `MarshalText`/`UnmarshalText` so it serializes identically to a plain
string but is type-checked at compile time. Prevents passing a step name where
a run ID is expected. No JSON break.

---

## Raw Ideas (not yet scoped)

- CLI tool (`auditlog`) for inspecting/replaying/diffing exported reports
- `encoding/json/v2` migration (Go 1.25+ policy mandate)
- `go-error-family` adoption for structured, classified errors
- `FailureReason` structured categories (typed categories, not just a string)
- `Diff()` on PeakConcurrency / CriticalPath (currently only duration)
- `ReplayEvents` round-trip property/fuzz test (export → read → replay = equivalent)
- Status report index page (`docs/status/INDEX.md`) linking the 5+ reports
- `CONTRIBUTING.md` documenting the HTML-vs-Markdown snapshot rule
- Configurable node shapes/icons per step type in diagrams
- Workflow-level retry/timeout surfacing in the report
