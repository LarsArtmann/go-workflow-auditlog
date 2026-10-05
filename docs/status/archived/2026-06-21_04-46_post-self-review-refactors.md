# Status Report — go-workflow-auditlog — 2026-06-21

**Generated**: 2026-06-21 04:46 CEST · **Module**: `github.com/larsartmann/go-workflow-auditlog` · **Go**: 1.26.3 · **Status**: ALPHA

---

## Build & Quality Gates

| Gate          | Status      | Detail                                    |
| ------------- | ----------- | ----------------------------------------- |
| Build         | ✅ PASS     | `go build ./...` exits 0                  |
| Tests         | ✅ PASS     | 165 pass, 0 fail (`-race`)                |
| Coverage      | ✅ 93.2%    | auditlog package                          |
| Lint          | ✅ 0 issues | `golangci-lint run ./...`                 |
| Vet           | ✅ PASS     | `go vet ./...` exits 0                    |
| Vulnerability | ✅ 0 vulns  | `govulncheck` — 0 in our code, 3 uncalled |

**Codebase**: 24 source files, 14 test files, ~7,535 LOC total. 12 direct deps, 28 transitive.

---

## A) FULLY DONE ✅

### Core Audit Pipeline

- `Auditor` with `Attach(w)` → `w.Do(ctx)` → `Snapshot(w)` → `Report()` lifecycle
- Per-attempt event capture: `attempt_start` / `attempt_end` with timestamps, errors, durations
- Full step DAG capture: dependencies, dependents, retry/timeout config, step types
- Sub-workflow traversal via `flow.Traverse` (captures inner steps that bypass callbacks)
- Skipped & canceled detection (reads post-execution state for steps that bypass Before/After)
- `StepInfo.Error` reflects FINAL outcome only (regression-tested)
- `MaxEvents` cap with `DroppedEventCount` tracking

### Report & Query API

- `WorkflowReport` with denormalized aggregates (counts, durations, peak concurrency, critical path)
- `Validate()` — checks count consistency (event, step, 6 status-count fields) + status drift via sentinel errors
- `Filtered(opts...)` — filter by step name, status, event type, time range
- `Diff(other)` — compare two runs (added/removed/changed steps + wall-clock duration delta)
- `Summary()` — one-line human-readable summary (uses wall-clock + failure reason)
- `Duration()` — wall-clock duration as `time.Duration`
- `ReportIndex` — O(1) lookup maps for repeated queries
- `ReplayEvents()` — reconstruct report from flat NDJSON event stream
- `LoadReport()` / `LoadReportFromReader()` / `LoadReportFromBytes()`
- `ReadEvents()` — NDJSON reader (inverse of WriteNDJSON)

### Report Aggregate Fields

- `WallClockDurationMs` — actual elapsed time (earliest → latest event)
- `PeakConcurrency` — max in-flight attempts (event-stream scan)
- `CriticalPathDurationMs` — longest dependency-chain duration (memoized DFS)
- `FailureReason` — human-readable failure summary
- `PendingCount` / `RunningCount` — split lifecycle-state counters

### Export Formats (all working, all tested)

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

### Architecture Quality

- **Single source of truth for colors**: `StepStatus.Color()` — all renderers delegate
- **Single DAG builder**: `buildGraph()` feeds Mermaid, Graphviz, PlantUML, D2
- **Single label function**: `stepLabel()` used by all renderers
- **Single report construction path**: `buildReportFromCore()` used by `BuildReport`, `Filtered`, `ReplayEvents`
- **Single step-state accumulator**: `stepCore` (shared by `stepRecord` live capture and replay) — **consolidated this session**
- **Branded RunID type**: `type RunID string` — compile-time identity safety, zero JSON break — **added this session**
- **Edges follow execution flow**: dependency → step (matches tree, GitHub Actions/Airflow convention)
- **D2 title derives from WorkflowID** (self-labeling, not hardcoded)
- **Concurrency model**: single `sync.RWMutex`, `atomic.Int64` for sequence, `OnEvent` fires outside lock

### Documentation

- `AGENTS.md` — comprehensive session context (file map, data flow, gotchas, testing patterns)
- `README.md` — end-user guide with API reference, examples, 3-duration-metrics explainer
- `CHANGELOG.md` — [Unreleased] populated with all changes
- `FEATURES.md` — honest feature inventory by status
- `TODO_LIST.md` — actionable short/mid-term tasks
- `ROADMAP.md` — long-term direction
- `docs/DOMAIN_LANGUAGE.md` — DDD glossary
- `example/main.go` — demos all export formats via `--export` flag
- `docs/reviews/2026-06-21_04-45_brutal-self-review.html` — self-review of architecture

### This Session's Refactors (4 commits)

1. **`computeWallClockDurationMs` moved to `report.go`** — finished cohesion job from prior session (was stranded in `report_builder.go`)
2. **`RunID` branded as typed string** — compile-time safety, zero JSON break. Updated all production + test code.
3. **`stepRecord`/`replayStep` consolidated into `stepCore`** — eliminated split brain. Single `toStepInfo()` conversion path. Live-only fields stay on `stepRecord`.
4. **AGENTS.md updated** with all three changes

---

## B) PARTIALLY DONE ⚠️

### 1. Deprecated Aliases Add Maintenance Surface

`WriteReportJSON`, `WriteEventsNDJSON`, `ExportToFile`, `ExportEventsToNDJSON` are deprecated aliases kept for backward compat. External tests still use them. Should be removed before v1.0 but kept until v0.2.0 consumers migrate.

### 2. Table Column Configuration

`buildTableData()` hardcodes 5 columns: Step, Status, Duration, Attempts, Error. No way for users to customize which columns appear or add custom columns. `HasRetry`, `HasTimeout`, `StepType` fields exist on `StepInfo` but aren't available as table columns.

### 3. Diagram Layout Direction

No way to set Mermaid/D2/Graphviz layout direction (TD vs LR). All diagrams default to top-down.

### 4. Error Stored as \*string

`Event.Error` and `StepInfo.Error` are `*string`, not `error`. Deliberate for JSON serialization but consumers lose `errors.Is`/`errors.As`. Changing this breaks the JSON schema.

### 5. \*float64 for DurationMs

`DurationMs *float64` appears in 5 places. The pointer distinguishes "not measured" (nil) from "zero" (0.0) but forces heap allocation and nil-checking everywhere. A typed wrapper would be cleaner.

---

## C) NOT STARTED ❌

| Item                               | Notes                                                                                                |
| ---------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `flake.nix`                        | Does not exist. Project uses `.goreleaser.yml` + deprecated `justfile`. AGENTS policy mandates nix.  |
| `encoding/json/v2` migration       | Go 1.25+ policy mandate. `v2` not yet in Go 1.26.3 stdlib — policy aspiration, not actionable today. |
| Table column configuration         | No way to customize columns.                                                                         |
| Configurable diagram direction     | No TD vs LR option.                                                                                  |
| `writeToFile` overwrite protection | `os.Create` truncates unconditionally. No `O_EXCL`.                                                  |
| Streaming NDJSON export            | Currently `Report()` materializes all events in memory.                                              |
| OpenTelemetry span bridge          | Map `attempt_end` events to OTel spans.                                                              |
| HTML dashboard report              | Self-contained HTML combining table + diagram + tree.                                                |
| Sub-module split                   | Core (JSON/NDJSON) vs visualization (diagrams/tables/trees). Decision needed.                        |
| Fuzz tests                         | No fuzz tests for diagram ID sanitization.                                                           |
| Benchmarks for new render paths    | WriteD2, WriteTable, WriteTree, `finalizeDenormalized` on 100+ steps.                                |
| Integration/round-trip tests       | report → JSON → LoadReport → report → diagram → verify.                                              |
| Cross-format consistency tests     | Same report → Mermaid vs DOT vs D2 should have same graph structure.                                 |

---

## D) TOTALLY FUCKED UP 🚨

### 1. Dependency Bloat: 12 Direct + 28 Transitive

The library went from 2 direct dependencies (go-workflow + backoff) to 12 direct + 28 transitive. The table module pulls in `charm.land/lipgloss/v2`. The serialization module pulls in `go-faster/yaml` + `go-faster/jx`. An auditlog library transitively requires a TUI styling framework.

**Root cause**: `table.go` blank-imports `go-output/table` (lipgloss) and `go-output/serialization` (yaml) via `init()` auto-registration. Every consumer now pays the dependency tax even if they only want JSON.

**Status**: Documented in ROADMAP.md. Architectural decision (sub-module split) deferred — needs user input on audience (lightweight audit trail vs full observability suite).

### 2. Error as Dead String, Not Structured Value

`Event.Error` and `StepInfo.Error` are `*string`. The error loses its type, its wrapped chain, and its `errors.Is`/`errors.As` capability. It's a dead string snapshot. This was a deliberate choice for JSON serialization, but it means consumers cannot programmatically classify errors.

**Status**: Changing this breaks the JSON schema. Ticketed in TODO_LIST.md.

---

## E) WHAT WE SHOULD IMPROVE

### Type Model

1. ~~**`StepID` is plain int** — could be branded for consistency with `RunID`. Low risk since it's internal.~~ **Won't implement — StepID kept int deliberately.**
2. ~~**`SchemaVersion` is plain string "0.1.0"** — should match the semver module tag. Currently diverges from module version.~~ **Won't implement — SchemaVersion decoupled from module tag.**
3. ~~**`DurationMs *float64`** — consider `time.Duration` with zero-as-absent, or a typed `Millis` wrapper.~~ *_Won't implement — _float64 deliberate — nil vs zero distinction.__
4. ~~**`go-error-family` adoption** — BuildFlow linter flags this. Sentinel errors are plain `errors.New`. Structured errors would enable classification.~~ done — go-error-family v0.5.0

### Architecture

~~5. **`writeToFile` silently truncates** — no overwrite protection. `O_EXCL` or a "file exists" check would prevent data loss.~~ done — CheckNoClobber + ErrFileExists v0.5.1
~~6. **Name collision warning** — when two steps share `String()`, the `seen` map silently merges them. Should warn.~~ done — NameCollisions v0.5.1
~~7. **`Name(step)` fallback helper** — `flow.String(step)` returns pointer addresses. A helper falling back to type name would give deterministic names.~~ done — HasPointerAddress v0.5.1

### Testing

8. ~~**Error-path coverage** — most Write\* methods sit at ~83% coverage. Inject failing `io.Writer`s.~~ done — error-path tests v0.5.0
9. ~~**`WritePlantUMLString` at 0% coverage** — noted in prior report, still untested.~~ done — 80% reached v0.5.0
10. ~~**Push coverage 93.2% → 95%+** — target `validateStatusCounts` branches and `omitempty` paths.~~ done — 97% reached 2026-10-05
11. ~~**Benchmarks for new render paths** — no perf regression detection for D2/table/tree.~~ done — benchmarks v0.5.1
12. ~~**Fuzz tests** — no fuzz coverage for diagram ID sanitization via go-output escape functions.~~ done — fuzz suite shipped

### Library Usage

13. ~~**`flake.nix` migration** — justfile deprecated per AGENTS policy. nix is the canonical build path.~~ done — flake.nix shipped
14. ~~**`govulncheck` in CI** — not automated. Run manually this session (0 vulns). Should be a gate.~~ done — govulncheck in CI
15. ~~**`gosec`** — never run. Security policy mandates it.~~ done — gosec enabled in .golangci.yml

---

## F) Top 25 Things to Get Done Next

Sorted by impact/effort ratio (highest first).

| #      | Task                                                                                                                | Impact       | Effort     | Category     |
| ------ | ------------------------------------------------------------------------------------------------------------------- | ------------ | ---------- | ------------ |
| ~~1~~  | ~~**Tag v0.2.0 release** — new public fields, API expansion, RunID type, edge-direction fix~~ done — v0.2.0 tagged  | ~~Critical~~ | ~~Low~~    | ~~Release~~  |
| ~~2~~  | ~~**Add error-path tests for all Write\* methods** — inject failing writers~~ done — error-path tests v0.5.0        | ~~High~~     | ~~Medium~~ | ~~Testing~~  |
| ~~3~~  | ~~**Add `WritePlantUMLString` coverage test** — currently 0%~~ done — 80% v0.5.0                                    | ~~High~~     | ~~Low~~    | ~~Testing~~  |
| ~~4~~  | ~~**Add `StepInfo.Type()` method** — expose StepType via method~~ done — StepInfo.Type() v0.5.1                     | ~~Medium~~   | ~~Low~~    | ~~API~~      |
| ~~5~~  | ~~**Add retry/timeout columns to table** — HasRetry/HasTimeout exist but not in table~~ done — columns v0.5.1       | ~~Medium~~   | ~~Low~~    | ~~Feature~~  |
| ~~6~~  | ~~**Push coverage 93.2% → 95%+** — target validateStatusCounts branches~~ done — 97% reached                        | ~~Medium~~   | ~~Medium~~ | ~~Testing~~  |
| ~~7~~  | ~~**Make table columns configurable** — column selection options~~ done — WithColumns v0.7.0                        | ~~Medium~~   | ~~Medium~~ | ~~Feature~~  |
| ~~8~~  | ~~**Add diagram layout direction option** — TD vs LR~~ done — WithDirection v0.7.0                                  | ~~Low~~      | ~~Low~~    | ~~Feature~~  |
| ~~9~~  | ~~**Add `writeToFile` overwrite protection** — O_EXCL flag~~ done — CheckNoClobber v0.5.1                           | ~~Low~~      | ~~Low~~    | ~~Safety~~   |
| ~~10~~ | ~~**Add integration/round-trip tests** — report → JSON → load → diagram → verify~~ done — integration tests v0.5.0  | ~~Medium~~   | ~~Medium~~ | ~~Testing~~  |
| ~~11~~ | ~~**Add cross-format consistency tests** — same report → all diagrams same graph~~ done — cross-format tests v0.5.0 | ~~Low~~      | ~~Medium~~ | ~~Testing~~  |
| ~~12~~ | ~~**Surface name collisions in diagrams** — warn when seen map merges~~ done — NameCollisions v0.5.1                | ~~Low~~      | ~~Medium~~ | ~~UX~~       |
| ~~13~~ | ~~**Offer `Name(step)` fallback helper** — type name when String() is pointer~~ done — HasPointerAddress v0.5.1     | ~~Medium~~   | ~~Low~~    | ~~API~~      |
| ~~14~~ | ~~**Add benchmarks for render paths** — WriteD2, WriteTable, WriteTree on 100+ steps~~ done — benchmarks v0.5.1     | ~~Low~~      | ~~Medium~~ | ~~Perf~~     |
| ~~15~~ | ~~**Add fuzz tests for diagram ID sanitization**~~ done — fuzz suite shipped                                        | ~~Low~~      | ~~Medium~~ | ~~Testing~~  |
| ~~16~~ | ~~**Add `flake.nix`** — migrate from deprecated justfile~~ done — flake.nix shipped                                 | ~~Medium~~   | ~~Medium~~ | ~~Infra~~    |
| ~~17~~ | ~~**Add `govulncheck` + `gosec` to CI**~~ done — govulncheck + gosec in CI                                          | ~~High~~     | ~~Low~~    | ~~Security~~ |
| ~~18~~ | ~~**Document table module `init()` pattern** in AGENTS.md gotchas~~ done — init() documented in AGENTS              | ~~Low~~      | ~~Low~~    | ~~Docs~~     |
| ~~19~~ | ~~**Add godoc `ExampleX` funcs** for WallClockDurationMs, PeakConcurrency, etc.~~ done — godoc examples shipped     | ~~Low~~      | ~~Low~~    | ~~Docs~~     |
| ~~20~~ | ~~**Consider sub-module split** — core (auditlog) + visualization~~ done — module split v0.8.0                      | ~~Medium~~   | ~~High~~   | ~~Arch~~     |
| ~~21~~ | ~~**Consider streaming NDJSON export** — write events as captured~~ done — streaming v0.8.0                         | ~~Low~~      | ~~High~~   | ~~Arch~~     |
| 22     | **Consider OpenTelemetry span bridge** **→ open — deferred (ROADMAP: OTel bridge, consumer-gated)**                 | Low          | High       | Feature      |
| ~~23~~ | ~~**Consider HTML dashboard report**~~ done — HTML dashboard v0.3.0                                                 | ~~Medium~~   | ~~High~~   | ~~Feature~~  |
| ~~24~~ | ~~**Consider `go-error-family` adoption**~~ done — go-error-family v0.5.0                                           | ~~Low~~      | ~~Medium~~ | ~~Arch~~     |
| ~~25~~ | ~~**Consider branded `StepID` type**~~ **Won't implement — StepID kept int deliberately.**                          | ~~Low~~      | ~~Low~~    | ~~Type~~     |

---

## G) The #1 Question I Cannot Answer Myself

**Should this library remain single-module with all go-output sub-modules as direct dependencies, or should it split into a core auditlog module (JSON/NDJSON only, 2 deps) and a visualization module (diagrams/tables/trees, go-output deps)?** **Resolved: split** — three modules shipped in v0.8.0 (core / viz / live).

The original design philosophy was "zero heavy deps" — 2 direct dependencies. The go-output adoption added 12 direct + 28 transitive including lipgloss (TUI styling), yaml, and toml parsers. This is fine if auditlog is meant to be a full-featured observability tool. But if consumers want a lightweight audit trail library (just JSON/NDJSON event capture), they now pay a heavy dependency tax for features they may not use.

The tradeoff: splitting into sub-modules (like go-output itself does) keeps the core dependency-light but adds module management complexity. Keeping single-module is simpler but forces all consumers to carry lipgloss + yaml. This is an **architectural direction decision, not a technical one** — I can implement either path, but I cannot decide which is right for the intended audience without knowing who consumes this library and how.
