# go-workflow-auditlog

Audit logging library for [Azure/go-workflow](https://github.com/Azure/go-workflow) — records every step execution event (attempts, retries, durations, errors, dependencies, final statuses) with timestamped events and export to JSON / NDJSON.

## Why?

go-workflow runs steps concurrently in a DAG, but provides no built-in way to answer:

- _Which step took the longest?_
- _How many retries did that flaky step need?_
- _What's the full dependency graph?_
- _Which steps were skipped, and why?_

This library answers those questions by injecting audit callbacks into your workflow and capturing a complete, timestamped event stream.

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    flow "github.com/Azure/go-workflow"
    "github.com/cenkalti/backoff/v4"

    "github.com/larsartmann/go-workflow-auditlog"
)

type FetchStep struct{ Data []byte }
func (s *FetchStep) Do(_ context.Context) error { s.Data = []byte("hello"); return nil }
func (s *FetchStep) String() string             { return "fetch" }

type SaveStep struct{ Input []byte }
func (s *SaveStep) Do(_ context.Context) error  { return nil }
func (s *SaveStep) String() string              { return "save" }

func main() {
    audit, _ := auditlog.New(auditlog.Config{
        Enabled:    true,
        WorkflowID: "my-pipeline",
    })

    fetch := &FetchStep{}
    save := &SaveStep{}

    w := &flow.Workflow{}
    w.Add(
        flow.Step(fetch),
        flow.Step(save).DependsOn(fetch),
    )

    // 1. Attach audit callbacks BEFORE running
    audit.Attach(w)

    // 2. Run the workflow
    _ = w.Do(context.Background())

    // 3. Snapshot final state AFTER running
    audit.Snapshot(w)

    // 4. Read the report
    report := audit.Report()
    fmt.Printf("Steps: %d, Events: %d\n", report.StepCount, report.EventCount)

    // Export
    _ = audit.ExportToFile("audit.json")
    _ = audit.ExportEventsToNDJSON("events.ndjson")
}
```

## How It Works

go-workflow v0.1.13 provides `BeforeStep` and `AfterStep` callbacks per step, fired per attempt (each retry try). This library:

1. **`Attach(w)`** — injects audit `BeforeStep`/`AfterStep` callbacks into every step in the workflow via `State.MergeConfig`. Must be called **before** `w.Do(ctx)`.

2. **Execution** — during `w.Do(ctx)`, each step's callbacks fire on every attempt, recording timestamped `attempt_start` and `attempt_end` events with duration, error, and status.

3. **`Snapshot(w)`** — reads the workflow's post-execution state to capture the full DAG structure, final statuses, retry/timeout config, and any steps that were **skipped or canceled** (which bypass callbacks entirely). Must be called **after** `w.Do(ctx)`.

### Why Snapshot is needed

Steps settled inline by Conditions (Skipped/Canceled) never enter the interceptor/callback chain. `Snapshot(w)` reads `w.StateOf(step)` and `w.UpstreamOf(step)` to fill these gaps.

## Report Structure

```json
{
  "version": "0.1.0",
  "workflow_id": "my-pipeline",
  "run_id": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
  "exported_at": "2026-06-18T15:21:09Z",
  "event_count": 4,
  "step_count": 2,
  "succeeded_count": 2,
  "failed_count": 0,
  "skipped_count": 0,
  "canceled_count": 0,
  "total_duration_ms": 15.23,
  "workflow_succeeded": true,
  "dropped_event_count": 0,
  "steps": [
    {
      "step_name": "fetch",
      "step_type": "FetchStep",
      "step_id": 1,
      "status": "succeeded",
      "attempt_count": 1,
      "duration_ms": 10.5,
      "has_retry": false,
      "has_timeout": false,
      "dependents": [{ "step_name": "save" }]
    }
  ],
  "events": [
    {
      "run_id": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
      "sequence": 1,
      "timestamp": "2026-06-18T15:21:09Z",
      "event_type": "attempt_start",
      "phase": "before",
      "step_name": "fetch",
      "attempt": 1
    }
  ]
}
```

## API Reference

### `auditlog.New(config Config) (*Auditor, error)`

Creates an auditor. When `Config.Enabled` is false, checks the `WORKFLOW_AUDITLOG_ENABLED` env var (`"true"`, `"1"`, `"yes"`).

### `Auditor` Methods

| Method                                                | Description                                                 |
| ----------------------------------------------------- | ----------------------------------------------------------- |
| `Attach(w *flow.Workflow) *flow.Workflow`             | Injects audit callbacks into all steps. Call before `Do`.   |
| `Snapshot(w *flow.Workflow)`                          | Captures final DAG state. Call after `Do`.                  |
| `Report() WorkflowReport`                             | Returns the consolidated report.                            |
| `Events() []Event`                                    | Returns all captured events.                                |
| `EventsCount() int`                                   | Event count without copying.                                |
| `DroppedEventCount() int64`                           | Events dropped due to `MaxEvents` cap.                      |
| `RunID() string`                                      | The run identifier stamped on every event (for correlation).|
| `ReportFiltered(opts ...ReportOption) WorkflowReport` | Returns a filtered report (by name/status/event-type/time). |
| `ExportToFile(path string) error`                     | Writes report as JSON.                                      |
| `ExportEventsToNDJSON(path string) error`             | Writes events as NDJSON.                                    |
| `WriteReportJSON(w io.Writer) error`                  | Writes report JSON to writer.                               |
| `WriteEventsNDJSON(w io.Writer) error`                | Writes NDJSON to writer.                                    |
| `ExportMermaid(path string) error`                    | Writes Mermaid DAG to file.                                 |
| `ExportPlantUML(path string) error`                   | Writes PlantUML DAG to file.                                |
| `ExportGraphviz(path string) error`                   | Writes Graphviz DOT DAG to file.                            |
| `WriteMermaid(w io.Writer) error`                     | Writes Mermaid DAG to writer.                               |
| `WritePlantUML(w io.Writer) error`                    | Writes PlantUML DAG to writer.                              |
| `WriteGraphviz(w io.Writer) error`                    | Writes Graphviz DOT DAG to writer.                          |

### `WorkflowReport` Methods

| Method                                                 | Description                                                          |
| ------------------------------------------------------ | -------------------------------------------------------------------- |
| `report.StepByName(name)`                              | Find a step by name.                                                 |
| `report.EventsByStep(name)`                            | Filter events by step.                                               |
| `report.EventsByType(type)`                            | Filter events by type.                                               |
| `report.FailedSteps()`                                 | All failed/canceled steps.                                           |
| `report.SucceededSteps()`                              | All succeeded steps.                                                 |
| `report.SkippedSteps()`                                | All skipped steps.                                                   |
| `report.RetriedSteps()`                                | All steps with >1 attempt.                                           |
| `report.Filtered(opts ...ReportOption) WorkflowReport` | Returns a filtered copy of the report.                               |
| `report.Diff(other WorkflowReport) DiffResult`         | Compares two reports (added/removed/changed steps + duration delta). |
| `report.Duration() time.Duration`                      | Wall-clock duration spanned by all events (earliest → latest).       |
| `report.Summary() string`                              | One-line human-readable summary.                                     |
| `report.WriteJSON(w io.Writer) error`                  | Serialize report as JSON.                                            |
| `report.WriteNDJSON(w io.Writer) error`                | Serialize events as NDJSON.                                          |
| `report.WriteMermaid(w io.Writer) error`               | Mermaid diagram.                                                     |
| `report.WritePlantUML(w io.Writer) error`              | PlantUML diagram.                                                    |
| `report.WriteGraphviz(w io.Writer) error`              | Graphviz DOT diagram.                                                |
| `report.WriteMermaidString() (string, error)`          | Mermaid diagram as string.                                           |
| `report.WritePlantUMLString() (string, error)`         | PlantUML diagram as string.                                          |
| `report.WriteGraphvizString() (string, error)`         | Graphviz DOT diagram as string.                                      |
| `report.Validate() error`                              | Checks internal consistency (counts, status drift).                  |

### Package-Level Functions

| Function                                                             | Description                                          |
| -------------------------------------------------------------------- | ---------------------------------------------------- |
| `auditlog.LoadReport(path string) (WorkflowReport, error)`           | Load a JSON report from a file.                      |
| `auditlog.LoadReportFromReader(r io.Reader) (WorkflowReport, error)` | Load a JSON report from a reader.                    |
| `auditlog.LoadReportFromBytes(b []byte) (WorkflowReport, error)`     | Load a JSON report from bytes.                       |
| `auditlog.ReadEvents(r io.Reader) ([]Event, error)`                  | Read NDJSON events (inverse of `WriteEventsNDJSON`). |
| `auditlog.ReplayEvents(events []Event) (WorkflowReport, error)`      | Reconstruct a report from a flat event stream.       |
| `auditlog.NewReportIndex(r WorkflowReport) *ReportIndex`             | Precompute O(1) lookup maps over a report.           |

### Sentinel Errors

These exported errors are returned by `Validate()` and `New()`. Match them with `errors.Is`:

| Error                              | Returned when                                        |
| ---------------------------------- | ---------------------------------------------------- |
| `auditlog.ErrWorkflowIDPathSep`    | `Config.WorkflowID` contains `/` or `\`.             |
| `auditlog.ErrEventCountMismatch`   | Report `EventCount` ≠ `len(Events)`.                 |
| `auditlog.ErrStepCountMismatch`    | Report `StepCount` ≠ `len(Steps)`.                   |
| `auditlog.ErrStatusDrift`          | A step's `Status` disagrees with its derived status. |
| `auditlog.ErrReplayNoEvents`       | `ReplayEvents` received zero events.                 |

## Config

| Field                  | Default                  | Description                                      |
| ---------------------- | ------------------------ | ------------------------------------------------ |
| `Enabled`              | `false` (checks env var) | Turns audit logging on/off.                      |
| `WorkflowID`           | `"default"`              | Human-readable identifier.                       |
| `RunID`                | auto-generated (128-bit hex) | Identifier for one execution; stamped on every event for trace correlation. Override to use your own trace ID. |
| `OnEvent`              | `nil`                    | Callback fired after each event. Must not block. |
| `MaxEvents`            | `0` (unlimited)          | Caps stored events to prevent OOM.               |
| `InitialEventCapacity` | `256`                    | Pre-allocates event slice.                       |

## Concurrency Model

- Single `sync.RWMutex` protects all mutable state.
- `BeforeStep`/`AfterStep` callbacks acquire the write lock once per call.
- `OnEvent` callback fires **outside** the lock to prevent user code from blocking.
- `BuildReport()` uses `RLock` — concurrent reads don't block each other.

## Step Naming

go-workflow uses `flow.String(step)` for display names. By default this returns `*TypeName(0xpointer)` which is non-deterministic. For clean audit output, implement `String()` on your step types:

```go
func (s *MyStep) String() string { return "my-meaningful-name" }
```

Or use `flow.Name(step, "name")` when adding to the workflow.

## Known Limitations

- **Step name collisions**: step identity is tracked internally by the `flow.Steper` pointer (always unique), but the JSON `step_name` field relies on `flow.String(step)`. If two steps produce the same `String()` output, their JSON output is ambiguous. Give each step a distinct `String()`.
- **Snapshot is mandatory for full DAG**: `Attach` captures per-attempt events; the dependency graph and skipped/canceled statuses are only filled in after `Snapshot(w)` reads the workflow's final state.
- **Replay loses DAG edges**: `ReplayEvents` reconstructs a report from a flat event stream. Dependencies, `MaxAttempts`, `HasRetry`, and `HasTimeout` are not available from events alone — use a Snapshot-captured report for full fidelity.
- **go-workflow retry data race**: `DefaultRetryOption.Backoff` shares a single `ExponentialBackOff` instance that races under concurrent use. This is a known upstream issue in v0.1.13.
- **`WorkflowID` cannot contain path separators** (`/` or `\`): it is used in export filenames and would break path safety. Use `Config.RunID` or your own export paths for arbitrary identifiers.

## Installation

```bash
go get github.com/larsartmann/go-workflow-auditlog
```

Requires Go 1.26+ and `github.com/Azure/go-workflow v0.1.13`.

## License

MIT
