# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- Initial project structure
- Diff API: `WorkflowReport.Diff(other) WorkflowReport` returns `DiffResult` with
  added/removed/changed step slices and a duration delta. StepDiff records both
  the new and previous status.
- Filter API: `WorkflowReport.Filtered(opts ...ReportOption)` for slicing reports
  by step name, status, event type, and time range.
- Replay API: `auditlog.ReplayEvents(events)` reconstructs a report from a flat
  event stream.
- Load API: `LoadReport`, `LoadReportFromReader`, `LoadReportFromBytes` parse
  JSON reports.
- Diagram export: Mermaid and PlantUML writers with status-based CSS classes.
- `ReadEvents` NDJSON reader with sentinel errors (`ErrEmpty`, `ErrNoEvents`,
  `ErrOversizedLine`).
- `Duration()` wall-clock helper and `Summary()` one-line report.
- Regression tests for fixed bugs (see Fixed).

### Changed

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

## [0.1.0] - 2026-01-01

### Added

- Initial release
