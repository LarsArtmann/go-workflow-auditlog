# Status Report — 2026-06-18 19:12

**Project**: `github.com/larsartmann/go-workflow-auditlog`
**Repo**: https://github.com/LarsArtmann/go-workflow-auditlog
**Branch**: master (tracks origin/master)
**Status**: ALPHA — feature-complete for v0.1.0, pushing to do a release cut

---

## Executive Summary

A library that adds audit logging, DAG visualization, and offline analysis to [Azure/go-workflow](https://github.com/Azure/go-workflow) v0.1.13. Sibling project to `samber-do-auditlog`, adapted for go-workflow's DAG execution model.

Built over 3 sessions from scratch. 5,233 LOC (16 source files + 11 test files), 112 tests, 94.7% coverage, race-clean. Pushed to GitHub.

---

## Metrics

| Metric          | Value                                                           |
| --------------- | --------------------------------------------------------------- |
| Source files    | 16 `.go` files + 1 example                                      |
| Test files      | 11 `_test.go` files                                             |
| Total LOC       | 5,233                                                           |
| Test count      | 112 (`=== RUN` lines)                                           |
| Benchmark count | 7 (14 sub-benchmarks)                                           |
| Coverage        | 94.7% of statements                                             |
| Race detector   | Clean                                                           |
| Commits         | 22                                                              |
| Dependencies    | `Azure/go-workflow v0.1.13`, `cenkalti/backoff/v4` (transitive) |

---

## a) FULLY DONE

### Core Library (production-ready)

- **`Auditor` public API** — `New(Config)`, `Attach(w)`, `Snapshot(w)`, `Report()`, `Events()`, `EventsCount()`, `DroppedEventCount()`
- **`Config`** — `Enabled`, `WorkflowID`, `OnEvent` callback, `MaxEvents` cap, `InitialEventCapacity`, env var fallback (`WORKFLOW_AUDITLOG_ENABLED`)
- **`Recorder`** — single-mutex state machine, atomic sequence counter, per-attempt tracking, defensive copies on read
- **`StepRef` type** — embedded in `Event` and `StepInfo` for JSON flattening; single source of truth for step identity `{Name, StepType}`
- **Sub-workflow support** — `snapshotWorkflow` uses `flow.Traverse` to walk the full step DAG, capturing inner steps of composite/sub-workflows

### Domain Model

- **`EventType`** — `attempt_start`, `attempt_end` (per-retry granularity)
- **`StepStatus`** — `pending`, `running`, `succeeded`, `failed`, `canceled`, `skipped` with `Label()`, `Icon()`, `IsTerminal()`, `IsError()` methods
- **`Event`** — timestamped observation with `Sequence`, `Attempt`, `DurationMs`, `Error`, `Status`
- **`StepInfo`** — full step snapshot with `Dependencies`/`Dependents` as `[]StepRef`, `AttemptCount`, `MaxAttempts`, `HasRetry`, `HasTimeout`, `DurationMs`
- **`WorkflowReport`** — consolidated report with denormalized aggregates, `Validate()`, `Reconstructed` flag

### Export Formats (6 formats)

- **JSON** — `Report.WriteJSON(w)`, `Auditor.ExportToFile(path)`, `Auditor.WriteReportJSON(w)`
- **NDJSON** — `Report.WriteNDJSON(w)`, `Auditor.ExportEventsToNDJSON(path)`, `Auditor.WriteEventsNDJSON(w)`
- **Mermaid** — `Report.WriteMermaid(w)`, `Report.WriteMermaidString()`, `Auditor.ExportMermaid(path)` — with status-colored CSS classes and retry indicators
- **PlantUML** — `Report.WritePlantUML(w)`, `Report.WritePlantUMLString()`, `Auditor.ExportPlantUML(path)`

### Import / Round-trip

- **NDJSON reader** — `ReadEvents(reader)` with sentinel errors (`ErrEmpty`, `ErrNoEvents`, `ErrOversizedLine`)
- **JSON loader** — `LoadReport(path)`, `LoadReportFromReader(r)`, `LoadReportFromBytes(b)`
- **Replay engine** — `ReplayEvents(events)` reconstructs a full `WorkflowReport` from a flat event stream

### Query & Analysis

- **Filter API** — `Report.Filtered(opts...)` with `WithStepsByName`, `WithStepsByStatus`, `WithEventsByType`, `WithTimeRange`
- **Diff API** — `Report.Diff(other)` returns `DiffResult` with added/removed/status-changed steps + duration delta
- **Query methods** — `StepByName`, `EventsByStep`, `EventsByType`, `FailedSteps`, `SucceededSteps`, `SkippedSteps`, `RetriedSteps`
- **Helpers** — `Report.Duration()` (wall-clock), `Report.Summary()` (one-line string)

### Testing & CI

- **112 tests** — covering disabled/enabled, success/failure, dependencies, retry, timeout/cancel, skip, concurrent, fan-out/fan-in, sub-workflows, event ordering, all export formats, replay round-trips, diff, filter, loader, nil safety
- **7 benchmark functions** (14 sub-benchmarks) — Invocation, Attach, BuildReport, EventsCopy, OnEventCallback, RetryWithAudit, MermaidExport
- **GitHub Actions CI** — 4 jobs: test (race), coverage gate (≥90%), lint (golangci-lint), mod-tidy drift check
- **golangci-lint config** — errcheck, gosimple, govet, ineffassign, staticcheck, unused, misspell, revive, gofmt, goimports

### Documentation

- **README.md** — quick start, API reference, config table, concurrency model, step naming guidance
- **AGENTS.md** — architecture, data flow, integration model, gotchas, testing patterns
- **godoc examples** — 4 testable `Example_*` functions
- **Example demo** — full data pipeline (fetch → validate → transform → save + retry + notify)

---

## b) PARTIALLY DONE

- **Mermaid diagram**: Functional with status coloring, but lacks edge labels (e.g. "depends on"), conditional branch visualization (`If`/`Switch`), and sub-workflow clustering. The samber-do sibling has a full HTML visualization; this project has no HTML export.
- **Replay engine**: Reconstructs status/duration/errors but cannot infer DAG dependencies (events carry no edge data). Documented as a known limitation.
- **`Filtered` events**: When filtering by step name/status, events are correctly scoped. But event-type filtering and time-range filtering don't combine with step filtering in an intuitive way (they're applied independently, then intersected).
- **Coverage**: 94.7% overall, but `WritePlantUMLString` has 0% coverage (never tested), and several error paths in `ReadEvents`, `matchEvent`, `writeToFile` are untested.

---

## c) NOT STARTED

- **HTML visualization** — samber-do sibling has a rich self-contained HTML export with CSS/JS, tabbed layout, interactive graph. Not started here.
- **Fuzz tests** — sibling has 3 fuzz targets (HTML XSS, migration, diagram escaping). None here.
- **OpenTelemetry bridge** — `contrib/otel` exists in go-workflow; no integration.
- **Migration/schema versioning** — sibling has `MigrateReport` for v0.1→v0.2. No schema migration path here (schema is v0.1.0, no prior versions to migrate from).
- **` FEATURES.md` / `TODO_LIST.md` / `CHANGELOG.md` / `STABILITY.md`** — not written.
- **`flake.nix` devShell** — sibling has Nix flake; this project has none.
- **Coverage gate at 95%** — CI gate is at 90%, sibling is at 95%.

---

## d) TOTALLY FUCKED UP

- **`go.mod` says `go 1.26.3`** — this is the patch version of the local toolchain, not a meaningful minimum. Should be `go 1.23` (go-workflow's minimum) to match CI.
- **`PlantUML.ClassAssign` is a no-op** — the `diagramFormatter` interface requires `ClassAssign`, but PlantUML returns `""`. This means the interface is leaky — it's designed around Mermaid's capabilities. Should either make `ClassAssign` optional or use a different abstraction.
- **`statusClass` doesn't cover `StepStatusRunning`** — a step that's still running (Snapshot called mid-execution) gets no CSS class in Mermaid output.
- **`stepTypeName` uses reflection** — `reflect.TypeOf(step)` on every step creation. For high-cardinality workflows this adds measurable overhead to the hot path. Could cache by type.
- **No `go.sum` verification in CI** — the `mod-tidy` job checks drift, but there's no `go mod verify` step.

---

## e) WHAT WE SHOULD IMPROVE

### Architecture

1. **`go.mod` go directive** — Change from `go 1.26.3` to `go 1.23` to match go-workflow's minimum and CI config.
2. **`diagramFormatter` interface** — The `ClassAssign` method is Mermaid-specific. Either split into a `MermaidExtras` interface or make the diagram engine handle absent classes gracefully.
3. **Type-safe step identity** — Currently `stepKey = flow.Steper` (an interface). Step names can collide if two steps have the same `String()` output. Consider a `StepID` type that combines pointer + name for disambiguation.
4. **`Recorder.workflowID` is write-only after construction** — No way to change it. If someone reuses the recorder across workflows, all events get the original ID. Consider making it mutable or documenting the constraint.

### Code Quality

5. **Reflection caching** — `stepTypeName` calls `reflect.TypeOf` on every step. Cache results by `reflect.Type` in a `sync.Map`.
6. **`sortByName` duplication** — Defined in `types.go` but inline `slices.SortFunc` used in `filter.go` and `report_builder.go`. Consolidate.
7. **Error sentinel coverage** — `Validate()` has 3 sentinel errors but `LoadReport` and `ReadEvents` errors are not sentinel-wrapped, making them hard to test with `errors.Is`.

### Testing

8. **`WritePlantUMLString` has 0% coverage** — Add at least one test.
9. **Error path coverage** — `ReadEvents` oversized line, `writeToFile` close error, `matchEvent` time-range edge cases.
10. **Fuzz tests** — Add `FuzzMermaidXSS` and `FuzzReplayIntegrity` targets.
11. **Integration test** — A single test that exercises the full lifecycle: Attach → Do → Snapshot → Report → Export JSON → Load JSON → Export NDJSON → Read NDJSON → Replay → Diff.

### Features

12. **HTML export** — Port the samber-do HTML template for a self-contained visual dashboard.
13. **`CHANGELOG.md`** — Document the v0.1.0 features for release.
14. **`flake.nix`** — Add Nix devShell for reproducible builds.
15. **OTel integration** — Bridge `OnEvent` to OpenTelemetry spans.

---

## f) Top #25 Things to Get Done Next

| #   | Task                                                               | Impact | Effort | Priority |
| --- | ------------------------------------------------------------------ | ------ | ------ | -------- |
| 1   | Fix `go.mod` go directive to `1.23`                                | High   | 1min   | P0       |
| 2   | Add `WritePlantUMLString` test (0% coverage)                       | Medium | 5min   | P0       |
| 3   | Write `CHANGELOG.md` for v0.1.0                                    | High   | 15min  | P0       |
| 4   | Cut v0.1.0 git tag + release                                       | High   | 5min   | P0       |
| 5   | Add `FEATURES.md` inventory                                        | Medium | 15min  | P1       |
| 6   | Add `TODO_LIST.md` from this report                                | Medium | 10min  | P1       |
| 7   | Add fuzz test for Mermaid XSS                                      | Medium | 30min  | P1       |
| 8   | Add fuzz test for replay integrity                                 | Medium | 30min  | P1       |
| 9   | Cache `stepTypeName` reflection results                            | Medium | 15min  | P1       |
| 10  | Add full lifecycle integration test                                | Medium | 30min  | P1       |
| 11  | Consolidate `sortByName` usage (remove duplication)                | Low    | 10min  | P1       |
| 12  | Fix `statusClass` to cover `StepStatusRunning`                     | Low    | 5min   | P1       |
| 13  | Add `go mod verify` to CI                                          | Low    | 5min   | P2       |
| 14  | Raise coverage gate from 90% to 95%                                | Low    | 30min  | P2       |
| 15  | Add error-path tests (`ReadEvents` oversized, `writeToFile` close) | Low    | 20min  | P2       |
| 16  | Add `STABILITY.md`                                                 | Low    | 10min  | P2       |
| 17  | Port HTML visualization from samber-do                             | High   | 2-4h   | P2       |
| 18  | Add `flake.nix` devShell                                           | Medium | 30min  | P2       |
| 19  | Split `ClassAssign` out of `diagramFormatter` interface            | Low    | 20min  | P2       |
| 20  | Add `go generate` support if templ is added                        | Low    | 15min  | P3       |
| 21  | Add OTel bridge example                                            | Medium | 1h     | P3       |
| 22  | Add Mermaid edge labels ("depends on")                             | Low    | 15min  | P3       |
| 23  | Add conditional branch (`If`/`Switch`) visualization               | Low    | 30min  | P3       |
| 24  | Add `Contributing.md`                                              | Low    | 10min  | P3       |
| 25  | Add `Code of Conduct`                                              | Low    | 5min   | P3       |

---

## g) Top #1 Question I Cannot Figure Out Myself

**Should the Mermaid diagram show the full sub-workflow tree flattened, or nest sub-workflow steps under a cluster/subgraph?**

Currently, `snapshotWorkflow` traverses sub-workflows with `flow.Traverse` and flattens all inner steps into a single `[]StepInfo`. This means the Mermaid diagram shows all steps at the same level — you lose the parent-child relationship between a composite step and its inner steps.

The alternative is to preserve the tree structure and render sub-workflows as Mermaid `subgraph` clusters. But go-workflow's `flow.Traverse` doesn't expose the depth/walked path in a way that makes reconstructing the tree trivial — the `walked []flow.Steper` parameter contains the traversal path, but mapping that to a clean cluster hierarchy requires understanding the wrapper types (`NamedStep`, `SubWorkflow`, etc.).

I don't know whether users of go-workflow commonly use sub-workflows, or whether flattened output is the 80% solution that's good enough. This affects the data model (do we need a `ParentStep` field on `StepInfo`?) and the diagram rendering.
