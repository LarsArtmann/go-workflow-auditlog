# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

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
