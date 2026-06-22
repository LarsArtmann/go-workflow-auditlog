# API Stability Promise (0.x)

> **Pre-1.0 notice.** This library is in ALPHA. The public API may change
> between minor releases. This document defines what you can rely on and what
> may evolve.

## Stable API (breaking changes require a major version bump or deprecation cycle)

These surfaces are used by every consumer and follow semantic versioning within
the 0.x series:

| Surface                                                                        | Contract                                                                      |
| ------------------------------------------------------------------------------ | ----------------------------------------------------------------------------- |
| `New(Config) Auditor`                                                          | Signature is stable. New `Config` fields may be added (zero-valued = opt-in). |
| `Auditor.Attach(w)` / `Auditor.Snapshot(w)` / `Auditor.Report()`               | Stable — this is the primary integration lifecycle.                           |
| `Auditor.Events()` / `Auditor.EventsCount()` / `Auditor.DroppedEventCount()`   | Stable.                                                                       |
| `Auditor.RunID()` / `RunID` type                                               | Stable. Branded string type, serializes as plain JSON string.                 |
| `ExportToFile`, `ExportEventsToNDJSON`, `Export*` methods                      | Stable method signatures. Output format may evolve.                           |
| `WorkflowReport` struct                                                        | Existing fields keep their JSON keys. New fields may be added.                |
| `ReadEvents`, `LoadReport`, `LoadReportFromReader`, `LoadReportFromBytes`      | Stable.                                                                       |
| `Config{Enabled, WorkflowID, RunID, MaxEvents, InitialEventCapacity, OnEvent}` | All current fields are stable. New fields may be added.                       |

## Evolving API (may change between 0.x releases)

These surfaces are functional but their exact shape may change:

| Surface                                         | Reason                                                  |
| ----------------------------------------------- | ------------------------------------------------------- |
| `WorkflowReport.Diff(other) DiffResult`         | `DiffResult` and `StepDiff` field sets may grow.        |
| `WorkflowReport.Filtered(opts ...ReportOption)` | The filter option set may expand.                       |
| `Event`, `StepInfo`, `StepRef` field set        | New fields may be added. Existing JSON tags are stable. |
| Diagram / Table / Tree export output            | Output format may evolve as go-output updates.          |
| `ReportIndex` query methods                     | New lookup methods may be added.                        |
| `ErrorClassifications()` / `RegisterClassifications(reg)` | Classification mapping may grow as new sentinels are added. |
| I/O sentinel errors (`ErrReportLoadFailed`, `ErrRenderFailed`, `ErrExportWriteFailed`) | Sentinel set is stable; wrapping messages may evolve. |
| `go-error-family` dependency (v0.5.0)           | Transitive: classification metadata depends on this external library. Pinned in go.mod. |

## Unstable / Internal (no stability guarantee)

- All unexported types and functions.
- The `stepRecord`, `stepCore` internal types.
- The `buildGraph` / `buildReportFromCore` / `statusStyle` internal functions.
