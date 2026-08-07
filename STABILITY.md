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
| `ExportJSON`, `ExportNDJSON`, `Export*` methods                                | Stable method signatures. Output format may evolve.                           |
| `WorkflowReport` struct                                                        | Existing fields keep their JSON keys. New fields may be added.                |
| `ReadEvents`, `LoadReport`, `LoadReportFromReader`, `LoadReportFromBytes`      | Stable.                                                                       |
| `Config{Enabled, WorkflowID, RunID, MaxEvents, InitialEventCapacity, OnEvent}` | All current fields are stable. New fields may be added.                       |

## Evolving API (may change between 0.x releases)

These surfaces are functional but their exact shape may change:

| Surface                                                                                   | Reason                                                                                    |
| ----------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `WorkflowReport.Diff(other) DiffResult`                                                   | `DiffResult` and `StepDiff` field sets may grow.                                          |
| `WorkflowReport.Filtered(opts ...ReportOption)`                                           | The filter option set may expand.                                                         |
| `Event`, `StepInfo`, `StepRef` field set                                                  | New fields may be added. Existing JSON tags are stable.                                   |
| Diagram / Table / Tree export output                                                      | Output format may evolve as go-output updates.                                            |
| `WithColumns(...TableColumn)` table option                                                | Column set may expand; existing column constants are stable.                              |
| `WithDirection(output.Direction)` diagram option                                          | Direction mapping per format may evolve; option signature is stable.                      |
| `TableColumn` enum / `DiagramOption` type                                                 | New values may be added; existing ones keep their iota/string values.                     |
| `ReportIndex` query methods                                                               | New lookup methods may be added.                                                          |
| `ErrorClassifications()` / `RegisterClassifications(reg)`                                 | Classification mapping may grow as new sentinels are added.                               |
| I/O sentinel errors (`ErrReportLoadFailed`, `ErrRenderFailed`, `ErrExportWriteFailed`)    | Sentinel set is stable; wrapping messages may evolve.                                     |
| `go-error-family` dependency (v0.10.0)                                                    | Transitive: classification metadata depends on this external library. Pinned in go.mod.   |
| `NDJSONStreamer` / `NewNDJSONStreamer` / `CreateNDJSONStreamer`                           | New streaming API; type and method set may grow. Output format (NDJSON) is stable.        |
| `WithAutoFlush()` / `WithBufferSize(n)` / `WithFlushInterval(d)` / `NDJSONStreamerOption` | Option set may expand; existing options keep their semantics.                             |
| `StreamEvents(reader, validate, fn)` / `StreamEventsCallback`                             | New streaming reader API; callback signature is stable.                                   |
| `NewMultiWriter(fn...)` / `MultiWriter` / `MultiWriterCallback`                           | New fan-out API; `MultiWriterCallback` matches `Config.OnEvent` signature.                |
| `Event.FailureReason` / `StepInfo.FailureReason` / `FailureReason` enum                   | New typed enum (`timeout`, `canceled`, `user_error`); new values may be added.            |
| `WorkflowReport.FailureSummary`                                                           | Renamed from `FailureReason` to avoid collision with event-level enum.                    |
| Workflow-level queries (`RetriedStepCount`, `TimedOutSteps`, `HasWorkflowRetries`, etc.)  | New aggregate methods on `WorkflowReport`; method set may grow.                           |
| `FailureReason.Label()` / `FailureReason.Color()` / `FailureReasonMeta`                   | New display metadata methods for visualizations.                                          |
| `ColumnFailureReason` table column                                                        | New `TableColumn` value; existing values keep their iota.                                 |
| Dashboard graph enhancements (critical path, retry badges, search, duration labels)       | JS post-processing layer on daghtml SVG; behavior may evolve with go-output daghtml SDK.  |
| `live.New(config, serverConfig)` / `live.Config` / `live.Server` / `live.Hub`             | New module; type and method set may change between 0.x releases.                          |
| `live.Server.SignalComplete()` / `live.Hub.OnEvent` / SSE event protocol                  | SSE event payloads (`snapshot`, `event`, `complete`) may gain fields; wire format stable. |
| `Auditor.CaptureDAG(w)`                                                                   | New method; pre-populates step DAG structure from workflow definition before execution.   |

### Live Server Enhancements (Evolving)

These surfaces are new and may change between 0.x releases:

| Surface                                                                   | Reason                                                                                                  |
| ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| `live.Config.Prefix`                                                      | Configurable route prefix (default "/"); URL construction may evolve.                                   |
| `live.Config.CORSAllowedOrigins`                                          | CORS header control (empty = disabled, secure-by-default; set to a specific origin or `"*"` to enable). |
| `live.Server` export endpoints (`/api/export/ndjson`, `/api/export/html`) | New endpoints; Content-Disposition headers may evolve.                                                  |
| `WorkflowReport.WriteCSV` / `WriteTSV` / `ExportCSV` / `ExportTSV`        | New delimited-value export; column set may expand.                                                      |

## Unstable / Internal (no stability guarantee)

- All unexported types and functions.
- The `stepRecord`, `stepCore` internal types.
- The `buildGraph` / `buildReportFromCore` / `statusStyle` internal functions.
