# AGENTS.md — go-workflow-auditlog

Go library for [Azure/go-workflow](https://github.com/Azure/go-workflow) that records every step execution event (attempts, retries, durations, errors, dependencies, final statuses) with timestamps and export to JSON / NDJSON.

**Module**: `github.com/larsartmann/go-workflow-auditlog` · **Package**: `auditlog` · **Go**: 1.26+ · **Status**: ALPHA

---

## Commands

| Command                                                         | Purpose                           |
| --------------------------------------------------------------- | --------------------------------- |
| `go test ./...`                                                 | Run all tests                     |
| `go test -race ./...`                                           | Run all tests with race detector  |
| `go test -race -coverprofile=cover.out -covermode=atomic ./...` | Tests with coverage (~94%)        |
| `go vet ./...`                                                  | Static analysis                   |
| `golangci-lint run ./...`                                       | Lint (golangci-lint v2, 0 issues) |
| `go run ./example`                                              | Run the demo pipeline             |

---

## Architecture

Single-package library (`auditlog`) with these source files:

```
doc.go             — Package doc comment
types.go           — Domain enums: EventType, Phase, StepStatus, StepRef, fromFlowStatus, SchemaVersion
event.go           — Event type (embeds StepRef, carries RunID) + convenience methods
step.go            — StepInfo type + stepCore (shared accumulator for live/replay) + toStepInfo() conversion
plugin.go          — Public API: New(), Attach(), Snapshot(), Report(), Export*(), Config, Auditor, RunID() + ErrExportWriteFailed sentinel + writeToFile()
recorder.go        — Core state machine: event capture, step records, attempt tracking, RunID + StepID counters
runid.go           — newRunID(): 128-bit crypto-random hex run identifier
attach.go          — Attach/Snapshot logic: callback injection + post-run DAG capture (incl. sub-workflows)
report.go          — WorkflowReport type (carries RunID) + Validate() + sentinel errors (incl. ErrRenderFailed) + query methods + Duration()/Summary() + WriteJSON + Export*() + computeWallClockDurationMs
report_builder.go  — BuildReport assembly: step records → sorted StepInfo + aggregates (WorkflowSucceeded, finalizeDenormalized)
filter.go          — Report filtering (Filtered, ReportOption, WithStepsByStatus, etc.)
diff.go            — Diff API: DiffResult/StepDiff, Diff() between reports
index.go           — ReportIndex: opt-in O(1) lookup maps over a report
loader.go          — LoadReport / LoadReportFromReader / LoadReportFromBytes + ErrReportLoadFailed sentinel
export.go          — NDJSON writer (writeEventsNDJSON internal helper)
ndjson.go          — ReadEvents NDJSON reader (sentinel errors, enum validation on ingest)
replay.go          — ReplayEvents: reconstruct Report from event stream (uses stepCore from step.go, preserves RunID + assigns StepIDs)
classify.go        — Error classification: RegisterClassifications() + ErrorClassifications() map sentinel errors → go-error-family Family (Corruption/Rejection) via init() auto-registration into DefaultRegistry
helpers.go         — Utility helpers: CheckNoClobber (anti-overwrite guard), HasPointerAddress (detect unoverridden String()), NameCollisions (find duplicate step names) + ErrFileExists sentinel
diagram.go         — Translation layer: buildGraph() converts WorkflowReport → go-output GraphNode/GraphEdge + statusStyle() maps StepStatus → GraphStyle colors
mermaid.go         — Mermaid flowchart export (delegates to go-output graph.MermaidRenderer, code fence off)
plantuml.go        — PlantUML component diagram export (delegates to go-output plantuml.PlantUMLDiagram)
graphviz.go        — Graphviz DOT digraph export (delegates to go-output graph.DOTRenderer, graphID "workflow")
d2.go              — D2 diagram export (delegates to go-output d2.D2Diagram, title "Workflow DAG")
table.go           — Step summary table export via go-output RenderTableData: table, json, csv, tsv, markdown, xml, d2, yaml, html, tree, mermaid, dot, jsonl, asciidoc, toml, plantuml
tree.go            — Step DAG tree export: ASCII tree (go-output/tree.ASCIITreeRenderer) + HTML nested list tree (go-output/markup.HTMLTreeRenderer)
metadata.go        — TypeMetadata struct + BuildTypeMetadata(): single source of truth for enum display metadata (icons/labels/colors) consumed by HTML dashboard JS
html.go            — Public API: WorkflowReport.WriteHTML / ExportHTML / WriteHTMLString (delegates to renderHTML)
html_render.go     — renderHTML(): assembles self-contained HTML dashboard via fmt.Sprintf on htmlTemplate; CSS/JS embedded from dashboard.css/dashboard.js via go:embed
dashboard.css      — Dashboard CSS theme (embedded at compile time via go:embed, ~550 lines)
dashboard.js       — Dashboard JavaScript: Sugiyama DAG graph engine, waveform renderer, tab switching, sortable tables, timeline, tree view (~714 lines)
example/           — Data pipeline demo (fetch → validate → transform → save + retry); version ldflags
.goreleaser.yml    — Automated GitHub releases with grouped changelog + demo binary
```

### Data Flow

1. User creates `Auditor` via `New(Config)`
2. `Attach(w)` injects `BeforeStep`/`AfterStep` callbacks into all steps via `State.MergeConfig`
3. During `w.Do(ctx)`, callbacks fire per-attempt → `Recorder` captures timestamped `Event`s
4. `Snapshot(w)` reads `w.StateOf(step)` + `w.UpstreamOf(step)` to fill in DAG structure and skipped/canceled statuses
5. `Report()` assembles `StepInfo` slice (with forward + reverse deps) and event stream
6. Export methods serialize to JSON, NDJSON, diagrams (Mermaid/PlantUML/DOT/D2), tables (16 formats via go-output), trees (ASCII/HTML), and interactive HTML dashboard (5-tab report with embedded CSS/JS) via [go-output](https://github.com/larsartmann/go-output)

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
- **writeToFile** is atomic (temp file + rename + bufio). A crash during export leaves the previous file intact, not a partial write.
- **NDJSON reader** validates enums on ingest: events with unknown `event_type` or `phase` are rejected with a descriptive error. Whitespace-only lines are skipped (not just empty lines).
- **Diagram export** uses [go-output](https://github.com/larsartmann/go-output) renderers. `buildGraph()` in `diagram.go` translates `WorkflowReport` steps into `output.GraphNode` + `output.GraphEdge` (edges point dependency → step, following execution flow — matching the tree export and the GitHub Actions / Airflow convention). `statusStyle()` maps `StepStatus` → `output.GraphStyle` fill colors (`succeeded`=`#2d5a2d` green, `failed`=`#8b2d2d` red, `skipped`=`#4a4a4a` gray, `canceled`=`#5a3d2d` orange). Mermaid uses per-node `style` directives (not `classDef`); code fence is OFF for raw `.mmd` output. DOT uses `graphID "workflow"`. PlantUML uses `[label] as id` component notation. D2 uses inline `style.fill`/`style.font-color` on nodes with `title: { label: Workflow DAG }`. Each renderer handles its own ID sanitization and label escaping — auditlog passes raw step names as node IDs. Dependency: `go-output` root v0.17.0 + sub-modules at v0.17.0 (graph, plantuml, d2, tree, table, markdown, markup, delimited, serialization; multi-module; all aligned since commit 4849d34).
- **Table export** (`WriteTable`) delegates to `output.RenderTableData` which dispatches to 16+ registered formats: table, json, csv, tsv, markdown, xml, d2, yaml, html, tree, mermaid, dot, jsonl, asciidoc, toml, plantuml. Sub-module imports auto-register renderers via `init()`. `buildTableData()` produces columns: Step, Status, Duration, Attempts, Error.
- **Tree export** (`WriteTree` / `WriteHTMLTree`) builds a `TreeNode` forest from step DAG: root = steps with no dependencies, children = dependents (execution flow). `tree.ASCIITreeRenderer` produces depth-colored ASCII output; `markup.HTMLTreeRenderer` produces nested `<ul>` lists.
- **`StepInfo.Error` reflects the FINAL outcome only.** For a step that fails on attempts 1–2 and succeeds on attempt 3, the `Error` field is `nil` (not "transient failure" from the last failed attempt). The per-attempt error history is preserved in the `Event` stream — each `attempt_end` event carries its own `Error`. Rationale: the step-level `Error` is the answer to "why did this step end in its final state?", and a succeeded step ended successfully. See `recorder.go:recordAfterStep` and the regression test `TestRetry_StepErrorClearedOnSuccess`.
- **`RunID` is a branded string type** (`type RunID string`) defined in `types.go`. It serializes to/from JSON as a plain string but the type system prevents accidentally passing a `WorkflowID` (also a string) where a `RunID` is expected. Convert with `RunID("value")` or `string(id)`. The `len()` built-in works directly on `RunID`; `hex.DecodeString` and similar stdlib functions need `string(runID)`.
- **`stepCore` is the shared step-state accumulator** (`step.go`) embedded by both `stepRecord` (live capture, `recorder.go`) and the replay accumulator (`replay.go`). The single `toStepInfo()` method produces the public `StepInfo` from the common fields. Live-only fields (stepID, pendingAttempts, maxAttempts, hasRetry, hasTimeout, dependencies) live on `stepRecord` only. Any new step-state field MUST go on `stepCore` so both paths stay synchronized.
- **HTML dashboard** (`html_render.go`) uses `go:embed` to embed `dashboard.css` and `dashboard.js` as separate files (proper syntax highlighting + linting). The `renderHTML()` function uses `fmt.Sprintf` on the `htmlTemplate` const (six `%s` verbs: CSS, version, report JSON, metadata JSON, JS, version). Report data is injected via `<script type="application/json">` tags (never parsed as HTML by the browser), and dynamic content in JS is escaped via the `esc()` function. Strict CSP: `default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'`. The JS Sugiyama graph engine (~200 lines) does rank assignment (Kahn's), 4-pass barycenter crossing reduction, and median-alignment positioning, then renders SVG with cubic bezier edges, pan/zoom/touch/click-highlight. `BuildTypeMetadata()` in `metadata.go` is the single source of truth for enum display metadata consumed by the JS. JS status colors reference CSS variables (`var(--success)` etc.) — no hardcoded hex duplication between CSS and JS. The diagram export hex colors in `types.go` (`stepStatusMeta`) are separate because diagram formats (Mermaid/DOT/D2) require literal hex values, not CSS variables. Tree root detection logic exists in both `tree.go` (Go, server-side tree export) and `dashboard.js` (client-side tree tab) — this is intentional: Go renders the tree export for static files, JS renders the interactive tab. Golden file test at `testdata/golden/report.html` — run `UPDATE_GOLDEN=1 go test -run TestReport_WriteHTML_GoldenFile` to update.
- **Error classification** (`classify.go`) registers all sentinel errors with [go-error-family](https://github.com/larsartmann/go-error-family) via `init()` auto-registration into `DefaultRegistry`. Consumers importing auditlog automatically get `errorfamily.Classify(err)`, `errorfamily.IsRetryable(err)`, and `errorfamily.ExitCode(err)` on auditlog errors. Strategy A (registration, not replacement) — sentinels stay as plain `error` values; `errors.Is` semantics are unchanged. Mapping: **Corruption** (exit 65, not retryable) = `ErrEventCountMismatch`, `ErrStepCountMismatch`, `ErrStatusDrift`, `ErrCountMismatch` (data integrity violations). **Rejection** (exit 1, not retryable) = `ErrEmpty`, `ErrNoEvents`, `ErrOversizedLine`, `ErrWorkflowIDPathSep`, `ErrReplayNoEvents` (bad caller input). **Transient** (exit 75, retryable) = `ErrReportLoadFailed` (retryable load/decode failures). **Infrastructure** (exit 69, not retryable) = `ErrRenderFailed` (rendering/marshaling), `ErrExportWriteFailed` (file write/flush/rename). Private sentinels `errUnknownEventType`/`errUnknownPhase` are also registered as Rejection. All 22 I/O error paths are wrapped with sentinels (loader, plugin/writeToFile, export, report.WriteJSON, mermaid/plantuml/graphviz/d2 render+write, table, tree, html_render marshal, html write). `ErrorClassifications()` returns the canonical map for consumer-side custom registries. `RegisterClassifications(*Registry)` registers into a custom registry for test isolation or scoped overrides. Unregistered errors default to Transient (fail-open for retry).

---

## Testing Patterns

- Standard `testing.T` + table-driven tests. No ginkgo/testify.
- Test steps implement `String()` for deterministic names.
- Retry tests use fresh `backoff.NewExponentialBackOff()` to avoid the go-workflow race.
- External test package (`auditlog_test`).
- `t.Setenv()` for env var tests (runs sequentially).
- 244 tests covering: disabled/enabled, success/failure, dependencies, retry, timeout/cancel, skip, concurrent steps, fan-out/fan-in, event ordering, OnEvent callback, export formats (JSON/NDJSON/D2/table/tree/HTML), report validation, query methods, filter, diff, replay, load, diagrams (Mermaid/PlantUML/DOT/D2), edge-direction consistency (diagrams vs tree), API symmetry (Write\*String on Auditor, Export\* on WorkflowReport — now including HTML), high-fan-out peak concurrency, diamond-DAG critical path, HTML from replayed/loaded reports, HTML diamond DAG + high fan-out, HTML structural integrity, HTML determinism, edge cases, error classification (all 12 public sentinels mapped to Family, wrapped error chain classification, custom registry registration, ExitCode/IsRetryable behavior, errors.Is identity preserved), **error-path tests** (failing io.Writer injection into all Write* methods, unwritable directories for Export*, invalid input for Load* — verifying errors.Is matches ErrRenderFailed/ErrExportWriteFailed/ErrReportLoadFailed on all 22 wrapped paths), plus regression tests for fixed bugs (status drift, diff ordering, NDJSON line numbers, WorkflowSucceeded honesty about pending steps, stale error cleared on retry success), **fuzz tests** (diagram special-char injection across Mermaid/PlantUML/DOT/D2; HTML XSS injection with structural integrity checks; Classify adversarial wrapped error chains), **property-based tests** (Diff algebra: identity, added/removed duality, duration anti-symmetry, status-change symmetry, sorted output — 200 iterations each, deterministic seeds; Classify wrapping-preserves-family through arbitrary depth, Classify identity matches ErrorClassifications map — 200 iterations), and **benchmarks** (runtime overhead: Invocation, Attach, BuildReport, EventsCopy, OnEventCallback, RetryWithAudit, MermaidExport; export rendering: WriteD2/WriteTable/WriteTree/WriteJSON/WriteMermaid on 100-step reports; renderHTML small 3-step + large 1000-step reports; godoc examples: Duration, Filtered).
- Coverage: **~94%** of statements (auditlog package).
- **Fuzz targets**: `FuzzDiagramSpecialChars` — diagram export structural integrity against injection payloads; `FuzzHTMLSpecialChars` — HTML dashboard XSS containment (12 seed payloads via step names, errors, dependency names) + structural integrity validation (balanced script tags, DOCTYPE, CSP).
- **Property tests**: 5 Diff algebra properties with 200 random report pairs each; HTML determinism (same report → identical output).
- **Benchmarks**: `BenchmarkRenderHTML_LargeReport` (1000 steps) + `BenchmarkRenderHTML_SmallReport` (3 steps).

### Shared test helpers (in `auditlog_test.go`)

| Helper                           | Purpose                                                       |
| -------------------------------- | ------------------------------------------------------------- |
| `mustNew`, `newAuditAndWorkflow` | Construct auditor + workflow fixtures                         |
| `retryOpts`, `addRetryStep`      | Wrap a step with retry config (fresh backoff)                 |
| `addParallelSteps`               | Wire two independent steps (no dependency edge)               |
| `addSlowParallelSteps`           | Wire two parallel slow steps of the same duration             |
| `addDependentStep`               | Wire a parent→child dependency chain                          |
| `addLinearChain`                 | Wire a 3-step linear chain (`a → b → c`)                      |
| `runWorkflow`                    | `Attach` + `Do` + `Snapshot` in one call                      |
| `findStep`, `assertReportValid`  | Step lookup + structural validation                           |
| `assertStepCount`                | Required step count (uses `Fatalf` to stop on mismatch)       |
| `assertEventCount`               | Required event count (`Errorf` — multiple counts may co-fail) |
| `assertCount(name, got, want)`   | Generic named-count assertion                                 |
| `assertWorkflowID`               | Required WorkflowID                                           |
| `assertAttemptCount`             | Required attempt count for a StepInfo                         |
| `assertStatus`                   | Required status for a StepInfo                                |
| `assertFirstStepName`            | Required name of `report.Steps[0]`                            |
| `assertContains`                 | `strings.Contains` check with custom failure message          |
| `assertEventsRecorded`           | `a.EventsCount() >= want` (loose event-count check)           |

### Duplicate-code policy

- Run `art-dupl --semantic --sort total-tokens -t 15` to find clones.
- Goal is **zero harmful duplication**, not zero report lines. Some
  signature-only matches (e.g. multiple `assert*(t, report, want)` helpers
  sharing the same parameter shape) are intentional: each helper asserts a
  different field with different semantics and merging would harm clarity.
- Production-code duplication is never acceptable: extract helpers (see
  `sortByName`, `sortStepsByName`, `diffStep`).

#### Acceptable clones (documented in source)

At `-t 15` ten clone groups remain after the deduplication pass; all are
classified as `idiom` by art-dupl and would only disappear at higher
thresholds (`-t 30` reports zero). Documented here so the next reader
knows they were considered.

- **`assert*` test helpers (auditlog_test.go)**: 7 helpers
  (`assertStepCount`, `assertEventCount`, `assertWorkflowID`,
  `assertFirstStepName`, `assertPeakConcurrency`, `assertFailureReason`,
  `assertFailedCount`) share the signature shape
  `func assertX(t *testing.T, report auditlog.WorkflowReport, want T)`.
  Each asserts a distinct field on the report with its own error message;
  merging into a generic `assertField(t, name, got, want)` would replace
  named call sites with stringly-typed lookups and harm test readability.
  Keeping typed helpers is the correct trade-off.

- **`WriteTable` API surface (plugin.go:246, table.go:52)** +
  **`WriteTableString` API surface (plugin.go:300, table.go:67)** +
  **`ExportTable` API surface (plugin.go:251, report.go:317)**: Each is a
  1-line delegating wrapper around the corresponding `WorkflowReport`
  method. The signature match is intrinsic to the consistent `Write*` /
  `Export*` method surface that `Auditor` exposes for every format
  (WriteMermaid, WritePlantUML, WriteGraphviz, WriteD2, WriteTree,
  WriteHTMLTree, WriteTable all follow the same delegation pattern).
  Removing the convenience methods would force callers to write
  `a.Report().Write*(...)` instead of `a.Write*(...)` for these formats
  only, breaking the API symmetry.

- **Workflow-fixture step literals (coverage_test.go, html_test.go)**:
  Two `StepRef: auditlog.StepRef{Name: "..."}` field literals inside
  larger `Steps: []auditlog.StepInfo{...}` literals. These are
  single-field struct initializers nested in composite test fixtures with
  many other fields (`Status`, `DurationMs`, `Dependencies`, etc.); the
  `StepRef` field is not the focal point of the assertion. Extracting a
  `stepRef("name")` helper would obscure the surrounding test data and
  add an indirection that is shorter than the field name itself.

- **`CriticalPathDurationMs` assertion (coverage_test.go)**: Two
  `if recomputed.CriticalPathDurationMs != want { t.Errorf(...) }` blocks
  with one providing a "diamond DAG:" prefix in the message for
  diagnostic context. The shared shape is the idiomatic Go
  `t.Errorf("expected X=%v, got %v", want, got)` pattern; merging would
  force callers to pass a context string for marginal benefit.

- **Test-table subtests for diagram formats (diagram_test.go:431-448)**:
  Two `assertEdgeDirections(t, "d2"/"graphviz", buf, expectedEdges,
forbiddenEdges)` calls with format-specific edge syntax
  (`fetch -> transform` vs `"fetch" -> "transform"`). This is the
  canonical table-driven test pattern: same harness, different data per
  subtest. Extracting would replace the existing helper call with a more
  parameterized one and reduce clarity.

- **`if err != nil { t.Fatalf }` in table-driven test (output_test.go)**:
  Two `t.Fatalf("%s X error: %v", tc.name, err)` calls inside the
  export-vs-string table of `TestExport_StringSymmetry`. The message
  encodes the operation ("export" vs "string") for diagnostic context;
  extracting would just produce `assertNoError(t, tc.name, op, err)` —
  adds parameter noise for a 1-line check.

- **`StepStatus` literal slice (diff_property_test.go:14, display_test.go:56)**:
  Two `[]auditlog.StepStatus{...}` literals enumerate the same six
  statuses but for different purposes: the diff-property test samples
  from the slice to build random reports; the display test iterates it
  to verify `String()` returns non-empty. Different domain use of the
  same enum membership; not duplication.

- **HTML file-contains assertions (html_test.go:137-189)**: Two
  `if !strings.Contains(string(data), "...") { t.Error(...) }` checks
  asserting different step-name substrings after reading a file written
  to disk. The two checks serve different subtests (`ExportHTML` and
  `Auditor.ExportHTML` delegation); the step names differ by design.
  Idiomatic file-content assertion; extracting a
  `assertFileContains(t, path, substr, msg)` helper would marginally
  shorten the call sites at the cost of an extra parameter.

#### Helper additions

- **`addLinearChain(w, a, b, c)`** (auditlog_test.go): wires a 3-step
  linear dependency chain (`a → b → c`) into a workflow. Centralizes
  the `w.Add(flow.Step(a), flow.Step(b).DependsOn(a),
flow.Step(c).DependsOn(b))` idiom previously duplicated across
  `diagram_test.go` and `html_test.go`. Companion to the existing
  `addDependentStep` (2-step chain) and `addParallelSteps` (no edges).
