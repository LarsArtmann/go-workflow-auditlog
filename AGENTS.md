# AGENTS.md — go-workflow-auditlog

Go library for [Azure/go-workflow](https://github.com/Azure/go-workflow) that records every step execution event (attempts, retries, durations, errors, dependencies, final statuses) with timestamps and export to JSON / NDJSON.

**Modules**: `github.com/larsartmann/go-workflow-auditlog` (core) · `github.com/larsartmann/go-workflow-auditlog/viz` (visualization) · **Go**: 1.26+ · **Status**: ALPHA

---

## Commands

| Command                                                                             | Purpose                                                  |
| ----------------------------------------------------------------------------------- | -------------------------------------------------------- |
| `GOEXPERIMENT=jsonv2 go test ./...`                                                 | Run core tests (requires jsonv2)                         |
| `GOEXPERIMENT=jsonv2 go test -race ./...`                                           | Run core tests with race detector                        |
| `GOEXPERIMENT=jsonv2 go test -race -coverprofile=cover.out -covermode=atomic ./...` | Core tests with coverage (~95.6%)                        |
| `GOEXPERIMENT=jsonv2 go vet ./...`                                                  | Core static analysis                                     |
| `golangci-lint run ./...`                                                           | Lint core (golangci-lint v2, 0 issues)                   |
| `cd viz && GOEXPERIMENT=jsonv2 go test ./...`                                       | Run viz tests (requires jsonv2)                          |
| `cd viz && GOWORK=off GOEXPERIMENT=jsonv2 go test ./...`                            | Run viz tests in standalone mode (no workspace)          |
| `cd viz && GOEXPERIMENT=jsonv2 go vet ./...`                                        | Viz static analysis                                      |
| `cd viz && golangci-lint run ./...`                                                 | Lint viz                                                 |
| `go run ./viz/example`                                                              | Run the demo pipeline                                    |
| `go run ./example`                                                                  | Run the demo pipeline (legacy path; now `./viz/example`) |
| `go work sync`                                                                      | Sync `go.work` and `go.work.sum` with module state       |

---

## Architecture

The project is split into two Go modules:

1. **Core module** (`github.com/larsartmann/go-workflow-auditlog`, package `auditlog`) —
   event recording, report construction, JSON/NDJSON serialization, replay, diff, filter, and index.
   It has no dependency on `go-output`.
2. **Visualization module** (`github.com/larsartmann/go-workflow-auditlog/viz`, package `viz`) —
   diagrams (Mermaid, PlantUML, Graphviz DOT, D2), tables (16 formats), trees (ASCII/HTML), and the
   interactive HTML dashboard. It depends on the core module and on `github.com/larsartmann/go-output`.

### Core module source files

```
doc.go             — Package doc comment
types.go           — Domain enums: EventType, Phase, StepStatus, StepRef, flowStatusMap, fromFlowStatus, SchemaVersion, AllStepStatuses, AllEventTypes
event.go           — Event type (embeds StepRef, carries RunID) + convenience methods
step.go            — StepInfo type + stepCore (shared accumulator for live/replay) + toStepInfo() conversion
plugin.go          — Public API: New(), Attach(), Snapshot(), Report(), WriteJSON/WriteNDJSON/ExportJSON/ExportNDJSON, Config, Auditor, RunID() + ErrExportWriteFailed sentinel
recorder.go        — Core state machine: event capture, step records, attempt tracking, RunID + StepID counters
runid.go           — newRunID(): 128-bit crypto-random hex run identifier
attach.go          — Attach/Snapshot logic: callback injection + post-run DAG capture (incl. sub-workflows)
report.go          — WorkflowReport type (carries RunID) + Validate() + sentinel errors (incl. ErrRenderFailed) + query methods + Duration()/Summary()/CriticalPath()/PeakConcurrencySteps() + stepsByName + WriteJSON/WriteNDJSON + ExportJSON/ExportNDJSON + computeWallClockDurationMs
report_builder.go  — BuildReport assembly: step records → sorted StepInfo + aggregates (WorkflowSucceeded, finalizeDenormalized) + computeCriticalPath/computePeakConcurrency/computePeakConcurrencySteps + sortEventsByTime
filter.go          — Report filtering (Filtered, ReportOption, WithStepsByStatus, etc.)
diff.go            — Diff API: DiffResult/StepDiff, Diff() between reports
index.go           — ReportIndex: opt-in O(1) lookup maps over a report
loader.go          — LoadReport / LoadReportFromReader / LoadReportFromBytes + ErrReportLoadFailed sentinel
export.go          — NDJSON writer (writeEventsNDJSON internal helper)
ndjson.go          — ReadEvents NDJSON reader (sentinel errors, enum validation on ingest)
replay.go          — ReplayEvents: reconstruct Report from event stream (uses stepCore from step.go, preserves RunID + assigns StepIDs)
stream.go          — NDJSONStreamer: real-time streaming NDJSON writer (thread-safe OnEvent callback, WithAutoFlush, CreateNDJSONStreamer)
classify.go        — Error classification: RegisterClassifications() + ErrorClassifications() map sentinel errors → go-error-family Family
helpers.go         — Utility helpers: CheckNoClobber, HasPointerAddress, NameCollisions + ErrFileExists sentinel + WriteToFile (atomic temp+rename export helper)
testhelpers/     — Exported test fixtures, step constructors, and assertions shared by both modules
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
daghtml_adapter.go  — buildDAGHTML() for the interactive HTML graph renderer
metadata.go         — TypeMetadata struct + BuildTypeMetadata() for the HTML dashboard JS
table.go            — WriteTable, WriteTableString, ExportTable
table_options.go    — TableColumn enum + WithColumns + DefaultTableColumns + AllTableColumns
tree.go             — WriteTree, WriteTreeString, ExportTree, WriteHTMLTree, WriteHTMLTreeString, ExportHTMLTree
html.go             — WriteHTML, WriteHTMLString, ExportHTML
html_render.go      — renderHTML(): assemble self-contained HTML dashboard
dashboard.css       — Dashboard CSS theme (embedded via go:embed)
dashboard.js        — Dashboard JavaScript: Sugiyama DAG, waveform, tabs, tables, timeline, tree
example/            — Data pipeline demo (now in viz module)
```

### Data Flow

1. User creates `Auditor` via `auditlog.New(Config)`
2. `Attach(w)` injects `BeforeStep`/`AfterStep` callbacks into all steps via `State.MergeConfig`
3. During `w.Do(ctx)`, callbacks fire per-attempt → `Recorder` captures timestamped `Event`s
4. `Snapshot(w)` reads `w.StateOf(step)` + `w.UpstreamOf(step)` to fill in DAG structure and skipped/canceled statuses
5. `Report()` assembles `StepInfo` slice (with forward + reverse deps) and event stream
6. Core consumers call `report.WriteJSON` / `report.WriteNDJSON` / `report.ExportJSON` / `report.ExportNDJSON`
7. Consumers who want visualization import `viz` and call `viz.WriteX(report, ...)` or `viz.ExportX(report, ...)`

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
- **Diagram/table/tree/HTML exports** live in the `viz` module and use [go-output](https://github.com/larsartmann/go-output) renderers. `buildGraph()` in `viz/diagram.go` translates `WorkflowReport` steps into `output.GraphNode` + `output.GraphEdge` (edges point dependency → step, following execution flow — matching the tree export and the GitHub Actions / Airflow convention). `statusStyle()` maps `StepStatus` → `output.NodeStyle` fill colors (`succeeded`=`#2d5a2d` green, `failed`=`#8b2d2d` red, `skipped`=`#4a4a4a` gray, `canceled`=`#5a3d2d` orange). Mermaid uses per-node `style` directives (not `classDef`); code fence is OFF for raw `.mmd` output. DOT uses `graphID "workflow"`. PlantUML uses `[label] as id` component notation. D2 uses inline `style.fill`/`style.font-color` on nodes with `title: { label: Workflow DAG }`. Each renderer handles its own ID sanitization and label escaping — auditlog passes raw step names as node IDs. **Layout direction** (`viz.WithDirection(output.Direction)`) is supported on all 4 diagram formats: DOT/D2 use native go-output `SetDirection()`; Mermaid post-processes `flowchart TD` → `flowchart LR/BT/RL`; PlantUML injects `left to right direction`. Direction helpers in `viz/diagram_options.go`. Dependency: `go-output` root v0.30.4 + sub-modules at v0.30.4 (graph, plantuml, d2, daghtml, tree, table, markdown, markup, delimited, serialization; multi-module; all aligned). The `.golangci.yml` exhaustruct exclude list must match the actual go-output struct names (`d2.Node`, `d2.NodeStyle`, `d2.StrokeStyle`, `d2.Edge`, `output.NodeStyle`) — not the old prefixed names (`D2Node` etc.).
- **Table export** (`viz.WriteTable`) delegates to `output.RenderTableData` which dispatches to 16+ registered formats: table, json, csv, tsv, markdown, xml, d2, yaml, html, tree, mermaid, dot, jsonl, asciidoc, toml, plantuml. Sub-module imports auto-register renderers via `init()`. **Column selection** via `viz.WithColumns(viz.TableColumn...)` — 10 columns available (Step, Status, Duration, Attempts, MaxAttempts, Retry, Timeout, Error, Type, Dependencies). `columnDefs` in `viz/table_options.go` is the single source of truth mapping `TableColumn` → header + extractor. Default = original 7 columns (backward compatible). Empty `WithColumns()` or no option → defaults.
- **Tree export** (`viz.WriteTree` / `viz.WriteHTMLTree`) builds a `TreeNode` forest from step DAG: root = steps with no dependencies, children = dependents (execution flow). `tree.ASCIITreeRenderer` produces depth-colored ASCII output; `markup.HTMLTreeRenderer` produces nested `<ul>` lists.
- **`StepInfo.Error` reflects the FINAL outcome only.** For a step that fails on attempts 1–2 and succeeds on attempt 3, the `Error` field is `nil` (not "transient failure" from the last failed attempt). The per-attempt error history is preserved in the `Event` stream — each `attempt_end` event carries its own `Error`. Rationale: the step-level `Error` is the answer to "why did this step end in its final state?", and a succeeded step ended successfully. See `recorder.go:recordAfterStep` and the regression test `TestRetry_StepErrorClearedOnSuccess`.
- **`RunID` is a branded string type** (`type RunID string`) defined in `types.go`. It serializes to/from JSON as a plain string but the type system prevents accidentally passing a `WorkflowID` (also a string) where a `RunID` is expected. Convert with `RunID("value")` or `string(id)`. The `len()` built-in works directly on `RunID`; `hex.DecodeString` and similar stdlib functions need `string(runID)`.
- **`stepCore` is the shared step-state accumulator** (`step.go`) embedded by both `stepRecord` (live capture, `recorder.go`) and the replay accumulator (`replay.go`). The single `toStepInfo()` method produces the public `StepInfo` from the common fields. Live-only fields (stepID, pendingAttempts, maxAttempts, hasRetry, hasTimeout, dependencies) live on `stepRecord` only. Any new step-state field MUST go on `stepCore` so both paths stay synchronized.
- **HTML dashboard** (`viz/html_render.go`) uses `go:embed` to embed `viz/dashboard.css` and `viz/dashboard.js` as separate files (proper syntax highlighting + linting). The `renderHTML()` function uses `fmt.Sprintf` on the `htmlTemplate` const (six `%s` verbs: CSS, version, report JSON, metadata JSON, JS, version). Report data is injected via `<script type="application/json">` tags (never parsed as HTML by the browser), and dynamic content in JS is escaped via the `esc()` function. Strict CSP: `default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'`. The JS Sugiyama graph engine (~200 lines) does rank assignment (Kahn's), 4-pass barycenter crossing reduction, and median-alignment positioning, then renders SVG with cubic bezier edges, pan/zoom/touch/click-highlight. `viz.BuildTypeMetadata()` in `viz/metadata.go` is the single source of truth for enum display metadata consumed by the JS. JS status colors reference CSS variables (`var(--success)` etc.) — no hardcoded hex duplication between CSS and JS. The diagram export hex colors in `types.go` (`stepStatusMeta`) are separate because diagram formats (Mermaid/DOT/D2) require literal hex values, not CSS variables. Tree root detection logic exists in both `viz/tree.go` (Go, server-side tree export) and `dashboard.js` (client-side tree tab) — this is intentional: Go renders the tree export for static files, JS renders the interactive tab. Golden content test (`TestReport_WriteHTML_GoldenContent` in `viz/html_golden_test.go`) validates structural + semantic content (DOCTYPE, CSP, 5 script tags, 5 tab panels, step names, WorkflowID, RunID, embedded CSS/JS, strict CSP policy) — NOT byte-for-byte comparison, which was retired after breaking 6+ times on whitespace/dependency drift.
- **Error classification** (`classify.go`) registers all sentinel errors with [go-error-family](https://github.com/larsartmann/go-error-family) via `init()` auto-registration into `DefaultRegistry`. Consumers importing auditlog automatically get `errorfamily.Classify(err)`, `errorfamily.IsRetryable(err)`, and `errorfamily.ExitCode(err)` on auditlog errors. Strategy A (registration, not replacement) — sentinels stay as plain `error` values; `errors.Is` semantics are unchanged. Mapping: **Corruption** (exit 65, not retryable) = `ErrEventCountMismatch`, `ErrStepCountMismatch`, `ErrStatusDrift`, `ErrCountMismatch` (data integrity violations). **Rejection** (exit 1, not retryable) = `ErrEmpty`, `ErrNoEvents`, `ErrOversizedLine`, `ErrWorkflowIDPathSep`, `ErrReplayNoEvents` (bad caller input). **Transient** (exit 75, retryable) = `ErrReportLoadFailed` (retryable load/decode failures). **Infrastructure** (exit 69, not retryable) = `ErrRenderFailed` (rendering/marshaling), `ErrExportWriteFailed` (file write/flush/rename). Private sentinels `errUnknownEventType`/`errUnknownPhase` are also registered as Rejection. All 22 I/O error paths are wrapped with sentinels (loader, plugin/WriteToFile, export, report.WriteJSON, viz diagram/table/tree/html render+write). `ErrorClassifications()` returns the canonical map for consumer-side custom registries. `RegisterClassifications(*Registry)` registers into a custom registry for test isolation or scoped overrides. Unregistered errors default to Transient (fail-open for retry).
- **Module split:** Core (`github.com/larsartmann/go-workflow-auditlog`) and visualization (`github.com/larsartmann/go-workflow-auditlog/viz`) are separate Go modules. The core module has no `go-output` dependency. Both modules share a `go.work` workspace in development; CI also verifies `GOWORK=off` standalone builds. The `testhelpers` package lives inside the core module so both core and viz tests can import it without a circular module dependency.

---

## Testing Patterns

- Standard `testing.T` + table-driven tests. No ginkgo/testify.
- Test steps implement `String()` for deterministic names.
- Retry tests use fresh `backoff.NewExponentialBackOff()` to avoid the go-workflow race.
- External test packages: `auditlog_test` for core, `viz_test` for visualization.
- Shared test helpers live in `testhelpers` (exported package inside the core module) and are imported by both core and viz tests.
- `t.Setenv()` for env var tests (runs sequentially).
- 319 test functions (288 tests, 15 benchmarks, 11 examples, 5 fuzz targets) covering: disabled/enabled, success/failure, dependencies, retry, timeout/cancel, skip, concurrent steps, fan-out/fan-in, event ordering, OnEvent callback, export formats (JSON/NDJSON/D2/table/tree/HTML), report validation, query methods, filter (combined type+time interaction), diff, replay, load, diagrams (Mermaid/PlantUML/DOT/D2), diagram direction (TD/LR/BT/RL across 4 formats), table column configuration (10 columns, custom selection, ordering, replayed reports), edge-direction consistency (diagrams vs tree), API symmetry (Write\*String functions in `viz`, Export\* functions in `viz`), high-fan-out peak concurrency, diamond-DAG critical path, CriticalPath() step chain, PeakConcurrencySteps(), HTML from replayed/loaded reports, HTML diamond DAG + high fan-out, HTML structural integrity, HTML golden content validation, HTML determinism, edge cases, error classification (all 12 public sentinels mapped to Family, wrapped error chain classification, custom registry registration, ExitCode/IsRetryable behavior, errors.Is identity preserved), **error-path tests** (failing io.Writer injection into all `viz.Write*` methods, unwritable directories for `viz.Export*`, invalid input for `Load*` — verifying errors.Is matches `ErrRenderFailed`/`ErrExportWriteFailed`/`ErrReportLoadFailed` on all wrapped paths), plus regression tests for fixed bugs (status drift, diff ordering, NDJSON line numbers, WorkflowSucceeded honesty about pending steps, stale error cleared on retry success), **fuzz tests** (diagram special-char injection across Mermaid/PlantUML/DOT/D2; multi-step diagram sanitization with unicode/control-char/keyword-collision seeds; HTML XSS injection with structural integrity checks; Classify adversarial wrapped error chains), **property-based tests** (Diff algebra: identity, added/removed duality, duration anti-symmetry, status-change symmetry, sorted output — 200 iterations each, deterministic seeds; Classify wrapping-preserves-family through arbitrary depth, Classify identity matches ErrorClassifications map — 200 iterations), and **benchmarks** (runtime overhead: Invocation, Attach, BuildReport, EventsCopy, OnEventCallback, RetryWithAudit, MermaidExport; export rendering: viz.WriteD2/viz.WriteTable/viz.WriteTree/viz.WriteJSON/viz.WriteMermaid on 100-step reports; renderHTML small 3-step + large 1000-step reports; godoc examples: Duration, Filtered, PeakConcurrency, CriticalPathDurationMs, WallClockDurationMs).
- Coverage: **~95.6%** of statements (auditlog package).
- **Fuzz targets**: `FuzzDiagramSpecialChars` — diagram export structural integrity against injection payloads; `FuzzDiagramSanitization_MultiStep` — multi-step diagram sanitization (17 seed pairs: unicode/emoji/CJK/Arabic, control chars, diagram-keyword collisions, whitespace-only, length extremes, edge sanitization) across Mermaid/PlantUML/DOT/D2; `FuzzHTMLSpecialChars` — HTML dashboard XSS containment (12 seed payloads via step names, errors, dependency names) + structural integrity validation (balanced script tags, DOCTYPE, CSP).
- **Property tests**: 5 Diff algebra properties with 200 random report pairs each; HTML determinism (same report → identical output).
- **Benchmarks**: `BenchmarkRenderHTML_LargeReport` (1000 steps) + `BenchmarkRenderHTML_SmallReport` (3 steps).

### Shared test helpers (in `testhelpers` package)

| Helper                                             | Purpose                                                                                 |
| -------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `NewSucceed`, `NewFail`, `NewFlaky`, `NewSlow`     | Construct test step instances                                                           |
| `SucceedStep`, `FailStep`, `FlakyStep`, `SlowStep` | Test step types exported for direct use in tests                                        |
| `StepFixture`                                      | Build a minimal `auditlog.StepInfo` for visualization/table tests                       |
| `RetryOpts`                                        | Build retry config with a fresh backoff instance                                        |
| `AddRetryStep`                                     | Wrap a step with retry config (fresh backoff)                                           |
| `AddSingleStep`                                    | Wire a single succeed step into a workflow                                              |
| `RunSingleSucceed`                                 | Run minimal single-succeed-step workflow (auditor + wf + step + Attach + Do + Snapshot) |
| `RunSingleSucceedWithBuffer`                       | `RunSingleSucceed` + fresh `*strings.Builder` for `Write*`-into-buffer tests            |
| `RunWorkflow`                                      | `Attach` + `Do` + `Snapshot` in one call                                                |
| `SingleSucceedExportPath`                          | `RunSingleSucceed` + `t.TempDir`-anchored path for `Export*` tests                      |
| `FindStep`, `AssertReportValid`                    | Step lookup + structural validation                                                     |
| `AssertStepCount`                                  | Required step count (uses `Fatalf` to stop on mismatch)                                 |
| `AssertEventCount`                                 | Required event count (`Errorf` — multiple counts may co-fail)                           |
| `AssertCount(name, got, want)`                     | Generic named-count assertion                                                           |
| `AssertWorkflowID`                                 | Required WorkflowID                                                                     |
| `AssertAttemptCount`                               | Required attempt count for a StepInfo                                                   |
| `AssertStatus`                                     | Required status for a StepInfo                                                          |
| `AssertFirstStepName`                              | Required name of `report.Steps[0]`                                                      |
| `AssertContains`                                   | `strings.Contains` check with custom failure message                                    |

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

At `-t 15` **zero** clone groups remain. The patterns previously listed here
(`Assert*` test helpers, `WriteTable`/`ExportTable` API-surface pairs, fixture
field literals, edge-direction assertions, etc.) were either eliminated by
the `writeGraph` + `SingleSucceedExportPath` extractions or were classified by
art-dupl as non-clones at the current threshold.

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
