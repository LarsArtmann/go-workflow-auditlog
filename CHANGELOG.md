# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- **`TableColumn.String()` method** — returns the column header name (e.g.
  `"Step"`, `"Max Attempts"`) for debug and logging ergonomics.

### Changed

- **Golden file test replaced with structural validation** — the fragile
  byte-for-byte golden file test (`TestReport_WriteHTML_GoldenFile`) that broke
  6+ times on whitespace/dependency drift is replaced by
  `TestReport_WriteHTML_GoldenContent`, which validates structural and semantic
  content (DOCTYPE, CSP, 5 balanced `<script>` tags, all 5 dashboard tab
  panels, JSON data blocks, embedded CSS/JS, step names, WorkflowID, RunID,
  schema version, strict CSP policy). No more `UPDATE_GOLDEN` maintenance.
- **`DefaultTableColumns` is now mutation-safe** — internal callers use
  `defaultColumnsCopy()` (fresh slice) so consumer code can never corrupt the
  package-level default. Documented as read-only.
- **README.md** — documented `WithColumns` and `WithDirection` features with a
  10-column reference table, diagram direction examples, and corrected
  WorkflowReport API signatures. Fixed stale badge values (234→319 tests,
  ~94%→~96% coverage).
- **AGENTS.md** — corrected test count (~310→319 with breakdown) and updated
  golden test reference to describe the new structural validation approach.

### Removed

- **`testdata/golden/report.html`** — retired the stale golden file that
  required constant regeneration. The test now validates structure and content
  without a committed reference file.

## [0.7.0] - 2026-07-22

### Added

- **Configurable table columns** — `WithColumns(TableColumn...)` option on
  `WriteTable`, `WriteTableString`, and `ExportTable`. Select from 10 columns
  (Step, Status, Duration, Attempts, MaxAttempts, Retry, Timeout, Error, Type,
  Dependencies). Default preserves backward compatibility (original 7 columns).
  Uses a pre-filter in `buildTableData` — no upstream go-output changes needed.
- **Diagram layout direction** — `WithDirection(output.Direction)` option on
  all diagram writers (`WriteMermaid`, `WriteGraphviz`, `WriteD2`,
  `WritePlantUML`) and their String/Export variants. Supports TD/LR/BT/RL
  directions. DOT and D2 use native go-output renderer support; Mermaid uses
  post-processing; PlantUML injects `left to right direction`.
- **`WorkflowReport.CriticalPath()`** — returns the ordered step chain
  (root-to-leaf) of the longest dependency path, not just the duration.
- **`WorkflowReport.PeakConcurrencySteps()`** — returns the unique steps that
  were in-flight at the moment of peak concurrency.
- **`FuzzDiagramSanitization_MultiStep`** — multi-step diagram fuzz target
  with 17 seed pairs (unicode, control chars, keyword collisions, length
  extremes, edge sanitization) across Mermaid/PlantUML/DOT/D2.
- **Public documentation website** at
  [go-workflow-auditlog.lars.software](https://go-workflow-auditlog.lars.software) —
  full Astro + Starlight site with 10 doc pages, landing page, and API reference.
- **Godoc `ExampleX` funcs** for `PeakConcurrency`, `CriticalPathDurationMs`,
  `WallClockDurationMs`, `WriteTable` (WithColumns), and `WriteMermaid`
  (WithDirection) — 7 runnable examples total.

### Changed

- **`fromFlowStatus` refactored** from switch to data-driven map lookup
  (`flowStatusMap`) for easier maintenance when go-workflow adds new statuses.
- **`coverage_test.go` split** into focused files: `coverage_types_test.go`,
  `coverage_report_test.go`, `coverage_plugin_test.go`.

- **`encoding/json/v2` migration** — all JSON serialization now uses Go's
  `encoding/json/v2` + `encoding/json/jsontext` packages (`GOEXPERIMENT=jsonv2`).
  Benefits: deterministic key ordering, ~2x faster marshaling, smaller
  allocations, and built-in HTML escaping for XSS hardening.
- **go-error-family bumped to v0.7.0** (was v0.5.0).
- **go-output bumped to v0.30.4** (was v0.30.1).
- **flake-parts + treefmt-nix** — migrated from flake-utils to flake-parts with
  treefmt-nix integration for build automation.
- **golangci-lint exhaustruct excludes** aligned with go-output v0.30.1 API
  (`d2.Node`, `d2.NodeStyle`, `d2.StrokeStyle`, `d2.Edge`, `output.NodeStyle`).
- **Build check added to flake.nix** to catch derivation errors early.

## [0.6.0] - 2026-07-06

### Changed

- **go-output upgraded to v0.30.1** (was v0.21.0) — brings daghtml SDK, refined
  graph/plantuml/d2 renderers, and tree/table/markup sub-module alignment.

## [0.5.1] - 2026-07-02

### Added

- **`StepInfo.Type()` method** — API consistency with `Status.Label()`/`Icon()`/`Color()`.
- **Retry & timeout columns in table export** — table now has 7 columns (Step,
  Status, Duration, Attempts, Retry, Timeout, Error).
- **Helper methods** — `CheckNoClobber(path)` (anti-overwrite guard),
  `HasPointerAddress(name)` (detect unoverridden `String()`),
  `WorkflowReport.NameCollisions()` (find duplicate step names).
- **`ErrFileExists` sentinel** — wraps `ErrExportWriteFailed` for clobber prevention.
- **Benchmarks** — runtime overhead (Invocation, Attach, BuildReport, EventsCopy,
  OnEventCallback, RetryWithAudit) + export rendering (WriteD2/Table/Tree/JSON/Mermaid
  on 100-step reports) + renderHTML (small 3-step + large 1000-step).
- **Integration / round-trip tests** — JSON round-trip + NDJSON replay round-trip.
- **Cross-format consistency tests** — Mermaid vs DOT vs D2 same nodes/edges.
- **Godoc `ExampleX` funcs** — `ExampleWorkflowReport_Duration` +
  `ExampleWorkflowReport_Filtered`.
- **ROADMAP strategic designs** — all 6 strategic items scoped with first-chunk designs.

### Changed

- **Go upgraded to 1.26.4** (was 1.26).
- **go-output upgraded** from v0.17.0 → v0.18.0 → v0.19.0 → v0.21.0 across this cycle.
- **HTML dashboard refactored** — replaced inline Sugiyama DAG JavaScript engine
  (~200 lines) with the [daghtml](https://github.com/larsartmann/go-output/daghtml)
  SDK from go-output. Smaller binary, maintained graph engine, same features
  (rank assignment, crossing reduction, pan/zoom/click-highlight).
- **Benchmark files consolidated** — fixed split brain where benchmarks were
  spread across multiple files.
- **HTML test helpers deduplicated** — removed shared helper duplication.

### Removed

- **Deprecated API aliases removed** — the four backward-compat aliases that
  were never in a released v1.0 are now deleted. Use the canonical names:
  - `WriteReportJSON` → `WriteJSON`
  - `WriteEventsNDJSON` → `WriteNDJSON`
  - `ExportToFile` → `ExportJSON`
  - `ExportEventsToNDJSON` → `ExportNDJSON`
- **makezero slice pre-allocation** replaced with idiomatic `append`.

## [0.5.0] - 2026-06-23

### Added

- **Error Classification via [go-error-family](https://github.com/larsartmann/go-error-family)** — all
  12 public sentinel errors (+ 2 private) are now classified into behavioral
  families (Corruption, Rejection, Transient, Infrastructure). Consumers can
  call `errorfamily.Classify(err)`, `errorfamily.IsRetryable(err)`, and
  `errorfamily.ExitCode(err)` on any auditlog error without additional setup
  (auto-registered via `init()`).
  - `ErrorClassifications()` — canonical `map[error]Family` (single source of truth)
  - `RegisterClassifications(reg)` — explicit registration into a custom registry
  - New dependency: `github.com/larsartmann/go-error-family v0.5.0`
- **3 new I/O sentinel errors** — all export/write/load paths now wrap matchable
  sentinels:
  - `ErrReportLoadFailed` (Transient, exit 75, retryable) — `LoadReport`,
    `LoadReportFromReader`, `LoadReportFromBytes`
  - `ErrRenderFailed` (Infrastructure, exit 69) — all render/marshal paths
    (`WriteJSON`, `WriteHTML`, `WriteTable`, diagrams, trees)
  - `ErrExportWriteFailed` (Infrastructure, exit 69) — all file-write paths
    (`Export*`, `WriteMermaid`, `WriteD2`, etc.)
- **22 I/O error paths wrapped** across 13 files — every `fmt.Errorf` in
  export/render/load paths now carries a matchable sentinel via `errors.Is`.

### Tests

- 18 error-path tests covering all Write*/Export*/Load\* sentinel wrapping paths
  with failing `io.Writer` injection and unwritable directories.

## [0.4.0] - 2026-06-22

### Added

- **Failure Summary banner** — prominent red banner at the top of the HTML
  dashboard when `workflow_succeeded` is false, showing `failure_reason` and
  per-step error messages for every failed/canceled step. Previously failure
  reasons were hidden behind click-only tooltips on status badges.
- **Workflow PASS/FAIL hero badge** — green checkmark or red cross next to the
  report title for at-a-glance status.
- **Error column in steps table** — inline truncated error text with full text
  on hover. Replaces the hidden tooltip-only approach.
- **Gantt-style timeline** — replaces the fake duration-sorted bar chart with a
  real time-positioned Gantt chart showing actual parallelism and step overlaps.
  Includes time axis labels.
- **Humanized durations** — all durations now display as `48.8s`, `5m 18s`,
  `2h 15m` instead of raw milliseconds.
- **Failure-impact badges** — "blocked" marker on steps skipped or canceled
  because a dependency failed, with the blocking step name.
- **Graph error dots** — red dot indicator on failed/canceled nodes in the SVG
  dependency graph.
- **Keyboard "e" shortcut** — toggles the "Errors only" filter in the steps
  table without mouse interaction.
- **"Errors only (N)" button** — shows count badge with red border when
  failures exist.
- **Inline errors in events table** — actual error text instead of a generic
  "error" badge.
- **Inline errors in DAG tree** — failed steps show error text directly in the
  tree node.
- **Color-coded failed rows** — subtle red-tinted background on failed/canceled
  rows in the steps and events tables.

### Changed

- **`esc()` and `humanizeDuration()` moved to top of `dashboard.js`** for
  readability — they are shared utilities called throughout.

### Removed

- **Dead timeline CSS** (`.timeline-bar`, `.timeline-row`, `.timeline-label`,
  `.timeline-track`) — left over from the old duration-sorted timeline, now
  replaced by the Gantt chart.

### Tests

- 9 new HTML feature tests (26 HTML tests total, 266 total tests, 92.9% coverage):
  failure banner (visible/hidden), error column, workflow status badge, Gantt
  chart, impact badge, humanized durations, graph failed-node dot, tree inline
  error.

## [0.3.0] - 2026-06-22

### Added

- **Interactive HTML dashboard** (`WriteHTML` / `ExportHTML` / `WriteHTMLString`) —
  self-contained 5-tab report (Steps table, DAG Tree, interactive SVG DAG Graph
  with Sugiyama layout + pan/zoom/click-highlight, Timeline bar chart, Events
  stream) with embedded CSS/JS via `go:embed`, no external dependencies. Strict
  CSP, XSS-tested via fuzz target. Available on both `Auditor` and
  `WorkflowReport`.
- **Type metadata injection** (`BuildTypeMetadata` / `metadata.go`) — single
  source of truth for enum display metadata (icons, labels, colors) consumed by
  the HTML dashboard's JavaScript.
- **Golden file test** (`html_golden_test.go`) — byte-stable HTML output test
  with `UPDATE_GOLDEN=1` pattern.
- **XSS fuzz target** (`FuzzHTMLSpecialChars`) — verifies XSS payloads are
  contained within JSON script tags and never rendered as executable HTML.
  Includes structural integrity validation (balanced tags, DOCTYPE, CSP).
- **HTML export tests** — replay-to-HTML, loaded-report-to-HTML, diamond DAG,
  high fan-out (10 steps), determinism, and structural integrity.
- **Benchmarks** — `BenchmarkRenderHTML_LargeReport` (1000 steps) and
  `BenchmarkRenderHTML_SmallReport` (3 steps).

### Changed

- **HTML dashboard architecture** — CSS and JS extracted from Go string
  constants to `dashboard.css` and `dashboard.js` files embedded via
  `go:embed`. The `renderHTML` function uses `fmt.Sprintf` on a template const
  instead of manual `strings.Builder` assembly. `html_render.go` shrank from
  1419 → 142 lines (90% reduction). Removes all `//nolint:lll` and
  `//nolint:funlen,varnamelen` suppressions.
- **API symmetry** — `WriteHTML` / `ExportHTML` added to table-driven
  `TestWorkflowReport_ExportMethods` and `TestAuditor_WriteStringMethods`.

## [0.2.1] - 2026-06-21

### Added

- **Nix flake support** — `flake.nix` + `flake.lock` for a reproducible dev shell
  and build environment.
- **`StepStatus.IsKnown()`** — enum validation helper for parity with
  samber-do-style unknown-value checks.
- **NDJSON enum validation on ingest** — `ReadEvents` now rejects events with
  unknown `event_type` or `phase` values instead of silently accepting invalid data.
- **Atomic file writes** — `writeToFile` writes to a temp file and renames, so a
  crash during export never leaves a partial file.
- **Fuzz tests** — `FuzzReadEvents` fuzz target for NDJSON parsing resilience.
- **Property tests** (`diff_property_test.go`) — 5 Diff algebra properties
  (identity, added/removed duality, duration anti-symmetry, status-change
  symmetry, sorted output) at 200 iterations each with deterministic seeds.
- **Coverage gate script** (`scripts/coverage-gate.sh`) — CI-enforced 92%
  statement-coverage floor.
- **Pre-commit hook** (`scripts/hooks/pre-commit`) — runs tests, vet, and lint
  before allowing commits.
- **STABILITY.md** — stability policy document.

### Changed

- **Enhanced CI** — coverage reporting, coverage-gate enforcement, race detector
  on all pushes, golangci-lint v2.4.0.

## [0.2.0] - 2026-06-21

### Added

- **Six new `WorkflowReport` aggregate fields**, all derived in the single
  `buildReportFromCore()` construction path so `BuildReport`, `Filtered`, and
  `ReplayEvents` stay consistent:
  - `WallClockDurationMs` — actual elapsed time (earliest → latest event),
    eliminating the up-to-4× inflation `TotalDurationMs` suffers for parallel
    workflows.
  - `PeakConcurrency` — maximum number of in-flight attempts at any instant
    (computed from the event stream).
  - `CriticalPathDurationMs` — longest dependency-chain duration (memoized DFS);
    the bottleneck path.
  - `FailureReason` — human-readable failure summary appended by `Summary()`.
  - `PendingCount` and `RunningCount` — split from the combined counter so each
    lifecycle state has its own denormalized field.
- **`ErrCountMismatch`** sentinel error returned by `Validate()` when a
  denormalized status-count field disagrees with the actual `Steps` slice.
- **D2 diagram export** (`WriteD2` / `WriteD2String` / `ExportD2`) via the
  [go-output](https://github.com/larsartmann/go-output) `d2` renderer.
- **Table export** (`WriteTable` / `ExportTable`) supporting 16 formats via
  `go-output.RenderTableData`: table, json, csv, tsv, markdown, xml, d2, yaml,
  html, tree, mermaid, dot, jsonl, asciidoc, toml, plantuml.
- **Tree export** — ASCII tree (`WriteTree` / `ExportTree`) and HTML nested-list
  tree (`WriteHTMLTree` / `ExportHTMLTree`).
- **Auditor file-export methods** for all new formats: `ExportD2`,
  `ExportTable`, `ExportTree`, `ExportHTMLTree`.
- **`Duration()`** method on `WorkflowReport` returning wall-clock elapsed time
  as a `time.Duration`.

### Changed

- **`Summary()` now reports wall-clock duration** and appends `FailureReason`
  on failure, making the one-liner self-diagnosing.
- **`Diff().DurationDelta` switched to wall-clock** so parallel-step
  regressions are no longer masked by summed durations.
- **`Validate()` now verifies all six status-count fields**
  (`Succeeded`/`Failed`/`Skipped`/`Canceled`/`Pending`/`Running`) against the
  actual `Steps` slice, extracted into `validateStatusCounts()`.
- **All diagram renderers migrated to go-output** — Mermaid, PlantUML, and
  Graphviz DOT now delegate to `graph.MermaidRenderer`,
  `plantuml.PlantUMLDiagram`, and `graph.DOTRenderer` respectively. A single
  `buildGraph()` translation layer feeds every diagram format.
- **Status colors consolidated** into `StepStatus.Color()` in `types.go` — all
  diagram renderers delegate to it (single source of truth).
- **`stepLabel` unified** — the diagram and tree label functions are now one
  shared `stepLabel()` that includes status text and retry indicator.

### Fixed

- **Diagram edge direction reversed to match execution flow.** Edges previously
  pointed step → dependency (backward against execution order), disagreeing
  with the tree export (which shows forward execution flow). Edges now point
  dependency → step, so diagrams and trees agree and arrows follow execution
  order — the convention used by GitHub Actions, Airflow, and Tekton.

## [0.1.1] - 2026-06-20

### Added

- **SECURITY.md** with a private vulnerability reporting process.

### Fixed

- **`StepInfo.Error` now reflects the final outcome.** Previously, when a step
  failed on early attempts and succeeded on a later retry, `StepInfo.Error`
  retained the error string from the last failed attempt — reporting a false
  "transient failure" even though `Status` was `Succeeded`. The recorder now
  always assigns the attempt error (a `nil` error clears any stale value), so
  the step-level `Error` always matches the final status. Per-attempt error
  history remains preserved in the `Event` stream — each `attempt_end` event
  carries its own `Error` pointer. Includes a regression test
  (`TestRetry_StepErrorClearedOnSuccess`).

## [0.1.0] - 2026-06-18

### Added

- Initial project structure.
- **Core audit pipeline**: `Auditor` with `Attach` → `Do` → `Snapshot` lifecycle,
  capturing per-attempt events (start/end, duration, error, status) and the full
  step DAG (dependencies, retry/timeout config, step types).
- **RunID**: every Event and WorkflowReport now carries a `run_id` identifying one
  workflow execution, for cross-system correlation (traces, logs). Auto-generated
  as a 128-bit hex ID, or set explicitly via `Config.RunID`. Read it via
  `Auditor.RunID()`. Survives NDJSON export → `ReplayEvents` round trips.
- **StepID**: each StepInfo gets a 1-based, unique `step_id` that disambiguates
  steps sharing the same name (identical `String()` output). Stable within a run.
- **Export formats**: JSON report (`ExportToFile`), NDJSON event stream
  (`ExportEventsToNDJSON`), and writer-based variants.
- **Diagram export**: Mermaid, PlantUML, and Graphviz DOT writers with
  status-based coloring. Includes `Export*` (file) and `Write*` (writer) variants.
- **Filter API**: `WorkflowReport.Filtered(opts ...ReportOption)` for slicing
  reports by step name, status, event type, and time range.
- **Diff API**: `WorkflowReport.Diff(other) DiffResult` returns added/removed/
  changed step slices and a duration delta. `StepDiff` records both the new and
  previous status.
- **Replay API**: `auditlog.ReplayEvents(events)` reconstructs a report from a
  flat event stream.
- **Load API**: `LoadReport`, `LoadReportFromReader`, `LoadReportFromBytes` parse
  JSON reports.
- **ReadEvents**: NDJSON reader with sentinel errors (`ErrEmpty`, `ErrNoEvents`,
  `ErrOversizedLine`).
- **ReportIndex**: `auditlog.NewReportIndex(report)` precomputes O(1) lookup maps
  (`StepByName`, `StepByID`, `EventsByStep`, `EventsByType`) for repeated queries.
- **Query helpers**: `Duration()` wall-clock span, `Summary()` one-line report,
  `FailedSteps()`, `SucceededSteps()`, `SkippedSteps()`, `RetriedSteps()`.
- **Exported sentinel errors**: `ErrWorkflowIDPathSep`, `ErrEventCountMismatch`,
  `ErrStepCountMismatch`, `ErrStatusDrift` — matchable via `errors.Is`.
- **goreleaser** config (`.goreleaser.yml`) for automated, changelog-grouped
  GitHub releases with a cross-compiled demo binary.
- **Acceptance tests**: end-to-end scenarios verifying mixed-outcome pipelines,
  OnEvent stream integrity, and the filter → export → load lifecycle.
- **Regression tests** for all fixed bugs (see Fixed below).
- CONTRIBUTING.md rewritten with commands, testing patterns, code style, and a
  documented release process.
- "Known Limitations" section in the README (name collisions, snapshot/replay
  constraints, upstream go-workflow retry race).

### Changed

- `NewRecorder` now takes a `runID` argument: `NewRecorder(workflowID, runID string, onEvent func(Event))`.
  Pre-1.0 breaking change to thread run identity to the event stream.
- `Config.OnEvent` doc corrected: the callback fires concurrently from parallel
  steps (not "sequentially") and must be goroutine-safe.
- `ReplayEvents` refactored into `replayStepsFromEvents` + `replayApplyEvent`
  helpers; replay output is now sorted deterministically and assigns StepIDs.
- `EventType` and `Phase` doc comments now explain their intentional redundancy.
- CI bumped to Go 1.26 and golangci-lint v2.4.0.
- `golangci.yml` overhauled: depguard now permits go-workflow and own module;
  tagliatelle configured for snake_case JSON; varnamelen / exhaustruct scopes
  narrowed; example/ and `_test.go` paths exempted from pedantic style rules.
- README and AGENTS.md refreshed with the full public API surface.
- DOMAIN_LANGUAGE.md filled in with the project's actual ubiquitous language.

### Deprecated

### Removed

- Orphan doc comments above `Auditor.Report()` (left over from when
  `Attach`/`Snapshot` lived in `plugin.go`).
- Unused `//nolint:exhaustruct` directives now that the linter config excludes
  own package types.
- Experimental Go build tags (`goexperiment.*`) from the linter config.

### Fixed

- `WorkflowReport.Validate()` status-drift check no longer dead code. The
  `step.Status.IsTerminal()` short-circuit made the check unreachable; it now
  fires whenever a step's stored Status disagrees with its Error pointer.
- `WorkflowReport.Diff()` output is now deterministic. Map iteration previously
  produced randomized `AddedSteps` / `RemovedSteps` / `StatusChanged` ordering;
  slices are now sorted by step name.
- `ReadEvents` now reports the true line number on parse errors (including
  skipped blank lines) instead of the event count.
- `WorkflowSucceeded` is now false when the report contains non-terminal steps.
  Previously, the absence of failures/cancels made it lie during mid-execution.
- `stepRecord.durationMs` is no longer clobbered by a stray `AfterStep` event
  that has no matching `BeforeStep` (a nil duration is now ignored).
- `WriteMermaidString` / `WritePlantUMLString` no longer double-wrap errors.
- `writeToFile` and `LoadReport` now wrap os errors with the offending path.
- `errTest` renamed to `testError` to satisfy the `errname` linter.
- Refactored `example/main.go` to reduce `main()` cyclomatic complexity from 17
  to under 10 by extracting `newAuditor`, `buildWorkflow`, `printReportSummary`,
  `printStepDetails`, `maybeExport`, and `printSampleEvent` helpers.

### Security

<!--
Release history:
  [0.1.0] - 2026-06-18 is the initial public release; it supersedes the
  placeholder "[0.1.0] - 2026-01-01" stub and consolidates all unreleased work.
-->
