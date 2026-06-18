# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

### Changed

### Fixed

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
