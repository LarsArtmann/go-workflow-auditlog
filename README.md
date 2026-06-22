# go-workflow-auditlog

[![Go Reference](https://pkg.go.dev/badge/github.com/larsartmann/go-workflow-auditlog.svg)](https://pkg.go.dev/github.com/larsartmann/go-workflow-auditlog)
[![Go Report Card](https://goreportcard.com/badge/github.com/larsartmann/go-workflow-auditlog)](https://goreportcard.com/report/github.com/larsartmann/go-workflow-auditlog)
[![CI](https://github.com/LarsArtmann/go-workflow-auditlog/actions/workflows/ci.yml/badge.svg)](https://github.com/LarsArtmann/go-workflow-auditlog/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-93.2%25-brightgreen)](#)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)

Audit logging library for [Azure/go-workflow](https://github.com/Azure/go-workflow) — records every step execution event (attempts, retries, durations, errors, dependencies, final statuses) with timestamped events and export to JSON / NDJSON.

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Example Output](#example-output)
- [How It Works](#how-it-works)
- [Report Structure](#report-structure)
- [API Reference](#api-reference)
- [Config](#config)
- [Diagrams](#diagrams)
- [HTML Dashboard](#html-dashboard)
- [Concurrency Model](#concurrency-model)
- [Step Naming](#step-naming)
- [Known Limitations](#known-limitations)
- [Contributing](#contributing)
- [License](#license)

---

## Features

- **Per-attempt event capture** — records every `attempt_start` / `attempt_end` with timestamp, duration, error, and status
- **Full DAG structure** — captures dependency graph (upstream + dependents), retry/timeout config, and step types
- **Skipped & canceled detection** — reads post-execution state to catch steps that bypass callbacks entirely
- **Cross-system correlation** — 128-bit `RunID` stamped on every event for trace/log correlation
- **Export formats** — JSON report, NDJSON event stream, Mermaid / PlantUML / Graphviz DOT / D2 diagrams, step summary tables (16 formats), ASCII + HTML tree views, **interactive HTML dashboard** (5-tab self-contained report with DAG graph engine, timeline, waveform)
- **Report filtering** — slice reports by step name, status, event type, or time range
- **Report diffing** — compare two runs for regression detection (added/removed/changed steps + duration delta)
- **Event replay** — reconstruct a report from a flat NDJSON event stream
- **O(1) lookups** — `ReportIndex` precomputes lookup maps for repeated queries
- **Sentinel errors** — matchable via `errors.Is` for programmatic branching
- **Zero runtime dependencies** — only `go-workflow` + `backoff/v4`
- **93.2% test coverage** with race detector, 0 lint issues

## Installation

```bash
go get github.com/larsartmann/go-workflow-auditlog
```

Requires Go 1.26+ and `github.com/Azure/go-workflow v0.1.13`.

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    flow "github.com/Azure/go-workflow"

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
    fmt.Println(report.Summary())

    // Export
    _ = audit.ExportJSON("audit.json")
    _ = audit.ExportNDJSON("events.ndjson")
}
```

The three-step lifecycle is **always** `Attach` → `Do` → `Snapshot`:

| Step       | When        | What it captures                                |
| ---------- | ----------- | ----------------------------------------------- |
| `Attach`   | Before `Do` | Injects audit callbacks into every step         |
| `Do`       | Execution   | Callbacks fire per-attempt, recording events    |
| `Snapshot` | After `Do`  | Reads DAG structure + skipped/canceled statuses |

## Example Output

Run the bundled demo:

```bash
go run ./example
```

```
━━━ demo version: dev | run id: 1dc74b1ad3d1b5c84e76097018e643f9 ━━━
  [audit] ▶ #1 attempt_start attempt=1 step=fetch
  [audit] ■ #2 attempt_end attempt=1 step=fetch (10.12ms)
  [audit] ▶ #3 attempt_start attempt=1 step=flaky-api-call
  [audit] ■ #4 attempt_end attempt=1 step=flaky-api-call error=transient error (0.01ms)
  [audit] ▶ #13 attempt_start attempt=2 step=flaky-api-call
  [audit] ■ #14 attempt_end attempt=2 step=flaky-api-call error=transient error (0.02ms)
  [audit] ▶ #15 attempt_start attempt=3 step=flaky-api-call
  → flaky step succeeded on attempt 3
  [audit] ■ #16 attempt_end attempt=3 step=flaky-api-call (0.02ms)
━━━ Workflow completed in 911.58ms ━━━

━━━ Audit Report ━━━
Workflow:     data-pipeline
Steps:        6
Succeeded:    6
Failed:       0
Events:       16
Total time:   28.53ms
Succeeded:    true

━━━ Step Details ━━━
  🟢 fetch [succeeded] attempts=1 type=FetchStep (10.12ms)
  🟢 flaky-api-call [succeeded] attempts=3 deps=[fetch] retry(max=5) error=transient error
  🟢 save [succeeded] attempts=1 type=SaveStep (5.12ms) deps=[transform]
```

## How It Works

go-workflow v0.1.13 provides `BeforeStep` and `AfterStep` callbacks per step, fired per attempt (each retry try). This library:

1. **`Attach(w)`** — injects audit `BeforeStep`/`AfterStep` callbacks into every step in the workflow via `State.MergeConfig`. Must be called **before** `w.Do(ctx)`.

2. **Execution** — during `w.Do(ctx)`, each step's callbacks fire on every attempt, recording timestamped `attempt_start` and `attempt_end` events with duration, error, and status.

3. **`Snapshot(w)`** — reads the workflow's post-execution state to capture the full DAG structure, final statuses, retry/timeout config, and any steps that were **skipped or canceled** (which bypass callbacks entirely). Must be called **after** `w.Do(ctx)`.

### Why Snapshot is needed

Steps settled inline by Conditions (Skipped/Canceled) never enter the interceptor/callback chain. `Snapshot(w)` reads `w.StateOf(step)` and `w.UpstreamOf(step)` to fill these gaps. Without it, the report is missing the dependency graph and non-executed step statuses.

### Why BeforeStep must pass through context

The `BeforeStep` callback signature is `func(ctx, Steper) (context.Context, error)`. The returned context flows into `step.Do(ctx)`. If the callback returns `context.Background()`, **step-level timeouts are destroyed**. This library returns the original `ctx` unchanged.

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
  "pending_count": 0,
  "running_count": 0,
  "total_duration_ms": 15.23,
  "wall_clock_duration_ms": 15.23,
  "workflow_succeeded": true,
  "dropped_event_count": 0,
  "peak_concurrency": 1,
  "critical_path_duration_ms": 15.23,
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

### Three Duration Metrics

The report carries three duration fields that answer different questions.
**Always use `wall_clock_duration_ms` for user-facing summaries and
regression detection.**

| Field                       | What it measures                                 | When it differs                                                                                                                                          |
| --------------------------- | ------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `total_duration_ms`         | Sum of every step's individual duration          | Inflated for parallel workflows — counts overlapping time multiple times. A 36-step pipeline running in 64&nbsp;s wall-clock can report 265&nbsp;s here. |
| `wall_clock_duration_ms`    | Actual elapsed time (earliest → latest event)    | The "how long did I wait?" number. Matches what a stopwatch would show. **Use this for summaries, `Summary()`, and `Diff()`.**                           |
| `critical_path_duration_ms` | Longest dependency-chain duration (memoized DFS) | The bottleneck path. If you can only parallelize one thing, this tells you which.                                                                        |

## API Reference

### `auditlog.New(config Config) (*Auditor, error)`

Creates an auditor. When `Config.Enabled` is false, checks the `WORKFLOW_AUDITLOG_ENABLED` env var (`"true"`, `"1"`, `"yes"`).

### `Auditor` Methods

| Method                                                | Description                                                  |
| ----------------------------------------------------- | ------------------------------------------------------------ |
| `Attach(w *flow.Workflow) *flow.Workflow`             | Injects audit callbacks into all steps. Call before `Do`.    |
| `Snapshot(w *flow.Workflow)`                          | Captures final DAG state. Call after `Do`.                   |
| `Report() WorkflowReport`                             | Returns the consolidated report.                             |
| `Events() []Event`                                    | Returns all captured events.                                 |
| `EventsCount() int`                                   | Event count without copying.                                 |
| `DroppedEventCount() int64`                           | Events dropped due to `MaxEvents` cap.                       |
| `RunID() string`                                      | The run identifier stamped on every event (for correlation). |
| `ReportFiltered(opts ...ReportOption) WorkflowReport` | Returns a filtered report (by name/status/event-type/time).  |
| `ExportJSON(path string) error`                       | Writes report as JSON.                                       |
| `ExportNDJSON(path string) error`                     | Writes events as NDJSON.                                     |
| `WriteJSON(w io.Writer) error`                        | Writes report JSON to writer.                                |
| `WriteNDJSON(w io.Writer) error`                      | Writes NDJSON to writer.                                     |
| `ExportMermaid(path string) error`                    | Writes Mermaid DAG to file.                                  |
| `ExportPlantUML(path string) error`                   | Writes PlantUML DAG to file.                                 |
| `ExportGraphviz(path string) error`                   | Writes Graphviz DOT DAG to file.                             |
| `ExportD2(path string) error`                         | Writes D2 DAG to file.                                       |
| `ExportTable(path string, format, opts) error`        | Writes step summary table (CSV/Markdown/JSON/HTML/...).      |
| `ExportTree(path string) error`                       | Writes ASCII tree to file.                                   |
| `ExportHTMLTree(path string) error`                   | Writes HTML tree to file.                                    |
| `ExportHTML(path string) error`                       | Writes interactive HTML dashboard to file.                   |
| `WriteMermaid(w io.Writer) error`                     | Writes Mermaid DAG to writer.                                |
| `WritePlantUML(w io.Writer) error`                    | Writes PlantUML DAG to writer.                               |
| `WriteGraphviz(w io.Writer) error`                    | Writes Graphviz DOT DAG to writer.                           |
| `WriteD2(w io.Writer) error`                          | Writes D2 DAG to writer.                                     |
| `WriteTable(w io.Writer, format, opts) error`         | Writes step summary table to writer.                         |
| `WriteTree(w io.Writer) error`                        | Writes ASCII tree to writer.                                 |
| `WriteHTMLTree(w io.Writer) error`                    | Writes HTML tree to writer.                                  |
| `WriteHTML(w io.Writer) error`                        | Writes interactive HTML dashboard to writer.                 |

### `WorkflowReport` Methods

| Method                                                  | Description                                                          |
| ------------------------------------------------------- | -------------------------------------------------------------------- |
| `report.StepByName(name)`                               | Find a step by name.                                                 |
| `report.EventsByStep(name)`                             | Filter events by step.                                               |
| `report.EventsByType(type)`                             | Filter events by type.                                               |
| `report.FailedSteps()`                                  | All failed/canceled steps.                                           |
| `report.SucceededSteps()`                               | All succeeded steps.                                                 |
| `report.SkippedSteps()`                                 | All skipped steps.                                                   |
| `report.RetriedSteps()`                                 | All steps with >1 attempt.                                           |
| `report.Filtered(opts ...ReportOption) WorkflowReport`  | Returns a filtered copy of the report.                               |
| `report.Diff(other WorkflowReport) DiffResult`          | Compares two reports (added/removed/changed steps + duration delta). |
| `report.Duration() time.Duration`                       | Wall-clock duration spanned by all events (earliest → latest).       |
| `report.Summary() string`                               | One-line human-readable summary.                                     |
| `report.WriteJSON(w io.Writer) error`                   | Serialize report as JSON.                                            |
| `report.WriteNDJSON(w io.Writer) error`                 | Serialize events as NDJSON.                                          |
| `report.WriteMermaid(w io.Writer) error`                | Mermaid diagram.                                                     |
| `report.WritePlantUML(w io.Writer) error`               | PlantUML diagram.                                                    |
| `report.WriteGraphviz(w io.Writer) error`               | Graphviz DOT diagram.                                                |
| `report.WriteD2(w io.Writer) error`                     | D2 diagram.                                                          |
| `report.WriteMermaidString() (string, error)`           | Mermaid diagram as string.                                           |
| `report.WritePlantUMLString() (string, error)`          | PlantUML diagram as string.                                          |
| `report.WriteGraphvizString() (string, error)`          | Graphviz DOT diagram as string.                                      |
| `report.WriteD2String() (string, error)`                | D2 diagram as string.                                                |
| `report.WriteTable(w, format, opts) error`              | Step summary table (16 formats via go-output).                       |
| `report.WriteTableString(format, opts) (string, error)` | Step summary table as string.                                        |
| `report.WriteTree(w io.Writer) error`                   | ASCII tree of step DAG.                                              |
| `report.WriteTreeString() (string, error)`              | ASCII tree as string.                                                |
| `report.WriteHTMLTree(w io.Writer) error`               | HTML nested-list tree of step DAG.                                   |
| `report.WriteHTMLTreeString() (string, error)`          | HTML tree as string.                                                 |
| `report.WriteHTML(w io.Writer) error`                   | Interactive HTML dashboard (5-tab report with DAG graph).            |
| `report.WriteHTMLString() (string, error)`              | HTML dashboard as string.                                            |
| `report.ExportHTML(path string) error`                  | Writes HTML dashboard to file.                                       |
| `report.Validate() error`                               | Checks internal consistency (counts, status drift).                  |

### Package-Level Functions

| Function                                                             | Description                                          |
| -------------------------------------------------------------------- | ---------------------------------------------------- |
| `auditlog.LoadReport(path string) (WorkflowReport, error)`           | Load a JSON report from a file.                      |
| `auditlog.LoadReportFromReader(r io.Reader) (WorkflowReport, error)` | Load a JSON report from a reader.                    |
| `auditlog.LoadReportFromBytes(b []byte) (WorkflowReport, error)`     | Load a JSON report from bytes.                       |
| `auditlog.ReadEvents(r io.Reader) ([]Event, error)`                  | Read NDJSON events (inverse of `WriteNDJSON`). |
| `auditlog.ReplayEvents(events []Event) (WorkflowReport, error)`      | Reconstruct a report from a flat event stream.       |
| `auditlog.NewReportIndex(r WorkflowReport) *ReportIndex`             | Precompute O(1) lookup maps over a report.           |

### Sentinel Errors

These exported errors are returned by `Validate()`, `New()`, `LoadReport()`, and export methods. Match them with `errors.Is`:

| Error                            | Returned when                                                        |
| -------------------------------- | -------------------------------------------------------------------- |
| `auditlog.ErrWorkflowIDPathSep`  | `Config.WorkflowID` contains `/` or `\`.                             |
| `auditlog.ErrEventCountMismatch` | Report `EventCount` ≠ `len(Events)`.                                 |
| `auditlog.ErrStepCountMismatch`  | Report `StepCount` ≠ `len(Steps)`.                                   |
| `auditlog.ErrStatusDrift`        | A step's `Status` disagrees with its derived status.                 |
| `auditlog.ErrCountMismatch`      | A denormalized status-count field disagrees with actual step counts. |
| `auditlog.ErrReplayNoEvents`     | `ReplayEvents` received zero events.                                 |
| `auditlog.ErrEmpty`              | `ReadEvents` input is empty.                                         |
| `auditlog.ErrNoEvents`           | `ReadEvents` input contains no events (all blank lines).             |
| `auditlog.ErrOversizedLine`      | `ReadEvents` line exceeds 1 MB.                                      |
| `auditlog.ErrReportLoadFailed`   | `LoadReport`/`LoadReportFromReader`/`LoadReportFromBytes` failed.    |
| `auditlog.ErrRenderFailed`       | Diagram/table/HTML/JSON rendering or marshaling failed.              |
| `auditlog.ErrExportWriteFailed`  | File write or I/O flush failed during export.                        |

### Error Classification

All sentinel errors are automatically registered with [go-error-family](https://github.com/larsartmann/go-error-family) on import. Consumers can call `errorfamily.Classify(err)`, `errorfamily.IsRetryable(err)`, or `errorfamily.ExitCode(err)` on any auditlog error without additional setup:

```go
import (
    "github.com/larsartmann/go-error-family"
    "github.com/larsartmann/go-workflow-auditlog"
)

err := auditlog.LoadReport("report.json")

if errorfamily.IsRetryable(err) {
    // Transient failure — back off and retry
}

switch errorfamily.Classify(err) {
case errorfamily.Corruption:
    // Data integrity violation (ErrEventCountMismatch, ErrStatusDrift, etc.)
case errorfamily.Rejection:
    // Bad caller input (ErrEmpty, ErrNoEvents, ErrOversizedLine, etc.)
case errorfamily.Transient:
    // Retryable failure (ErrReportLoadFailed)
case errorfamily.Infrastructure:
    // System-level failure (ErrRenderFailed, ErrExportWriteFailed)
}
```

| Family | Retry? | Exit | Sentinel errors                                              |
|--------|--------|------|--------------------------------------------------------------|
| Corruption | No | 65 | `ErrEventCountMismatch`, `ErrStepCountMismatch`, `ErrStatusDrift`, `ErrCountMismatch` |
| Rejection | No | 1 | `ErrEmpty`, `ErrNoEvents`, `ErrOversizedLine`, `ErrWorkflowIDPathSep`, `ErrReplayNoEvents` |
| Transient | Yes | 75 | `ErrReportLoadFailed` |
| Infrastructure | No | 69 | `ErrRenderFailed`, `ErrExportWriteFailed` |

For custom registries, call `auditlog.RegisterClassifications(reg)` explicitly.

## Config

| Field                  | Default                      | Description                                                                                                    |
| ---------------------- | ---------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `Enabled`              | `false` (checks env var)     | Turns audit logging on/off.                                                                                    |
| `WorkflowID`           | `"default"`                  | Human-readable identifier.                                                                                     |
| `RunID`                | auto-generated (128-bit hex) | Identifier for one execution; stamped on every event for trace correlation. Override to use your own trace ID. |
| `OnEvent`              | `nil`                        | Callback fired after each event. Fires concurrently — must be goroutine-safe.                                  |
| `MaxEvents`            | `0` (unlimited)              | Caps stored events to prevent OOM. Excess counted in `DroppedEventCount`.                                      |
| `InitialEventCapacity` | `256`                        | Pre-allocates event slice.                                                                                     |

## Diagrams

Export the step DAG in three formats, with status-based coloring (succeeded = green, failed = red, skipped = gray, canceled = orange):

**Mermaid** (renders natively in GitHub):

```mermaid
flowchart TD
    fetch[fetch]
    validate[validate]
    transform[transform]
    save[save]
    fetch --> validate
    validate --> transform
    transform --> save

    %% Styling
    style fetch fill:#2d5a2d,color:#fff
    style validate fill:#2d5a2d,color:#fff
    style transform fill:#2d5a2d,color:#fff
    style save fill:#2d5a2d,color:#fff
```

**Graphviz DOT**:

```dot
digraph workflow {
  rankdir=TB;
  node [shape=box fontname="Helvetica"];

  "fetch" [label="fetch" fillcolor=#2d5a2d];
  "validate" [label="validate" fillcolor=#2d5a2d];
  "transform" [label="transform" fillcolor=#2d5a2d];
  "save" [label="save" fillcolor=#2d5a2d];

  "fetch" -> "validate";
  "validate" -> "transform";
  "transform" -> "save";
}
```

Generate diagrams from any report:

```go
_ = report.WriteMermaid(os.Stdout)      // or audit.ExportMermaid("dag.mmd")
_ = report.WriteGraphviz(os.Stdout)     // or audit.ExportGraphviz("dag.dot")
_ = report.WritePlantUML(os.Stdout)     // or audit.ExportPlantUML("dag.puml")
_ = report.WriteD2(os.Stdout)           // or audit.ExportD2("dag.d2")
_ = report.WriteTable(os.Stdout, output.FormatCSV, output.RenderOptions{})  // step summary table
_ = report.WriteTree(os.Stdout)         // ASCII tree of step DAG
_ = report.WriteHTMLTree(os.Stdout)     // HTML nested-list tree
_ = audit.ExportHTML("dashboard.html")  // interactive HTML dashboard
```

## HTML Dashboard

The `WriteHTML` / `ExportHTML` methods produce a **self-contained interactive HTML dashboard** — a single file with embedded CSS and JavaScript, no external dependencies. Open it in any browser or attach it to a report/email.

**Five tabs:**

1. **Steps** — sortable, filterable table with pagination (step name, type, status, attempts, duration, dependencies, dependents, retry/timeout config)
2. **DAG Tree** — collapsible tree of the step dependency graph (root = no dependencies, children = dependents)
3. **DAG Graph** — interactive SVG with Sugiyama layered layout (pan/zoom/touch/click-to-highlight), built from the same DAG as the other exports
4. **Timeline** — horizontal bar chart of step durations, sorted by duration, color-coded by status
5. **Events** — sortable event stream with type filters and pagination

**Security:** Report data is injected via `<script type="application/json">` tags (never parsed as HTML). Dynamic content is escaped via a JS `esc()` function. Strict CSP: `default-src 'none'`. XSS-tested via fuzz target `FuzzHTMLSpecialChars`.

```go
// From an Auditor (live workflow):
audit.ExportHTML("dashboard.html")

// From a WorkflowReport (e.g. loaded from JSON):
report.ExportHTML("dashboard.html")

// To a string:
html, _ := report.WriteHTMLString()
```

## Concurrency Model

- Single `sync.RWMutex` protects all mutable state (`events`, `steps`, `stepCounter`).
- `BeforeStep`/`AfterStep` callbacks acquire the write lock once per call.
- `OnEvent` callback fires **outside** the lock — but **concurrently** from parallel step goroutines. It must be goroutine-safe.
- `BuildReport()` uses `RLock` — concurrent reads don't block each other.

## Step Naming

go-workflow uses `flow.String(step)` for display names. By default this returns `*TypeName(0xpointer)` which is non-deterministic across runs. For clean audit output, implement `String()` on your step types:

```go
func (s *MyStep) String() string { return "my-meaningful-name" }
```

Or use `flow.Name(step, "name")` when adding to the workflow.

## Known Limitations

- **Step name collisions**: step identity is tracked internally by the `flow.Steper` pointer (always unique), but the JSON `step_name` field relies on `flow.String(step)`. If two steps produce the same `String()` output, their JSON output is ambiguous. Use the `step_id` field (1-based, stable per run) to disambiguate, or give each step a distinct `String()`.
- **Snapshot is mandatory for full DAG**: `Attach` captures per-attempt events; the dependency graph and skipped/canceled statuses are only filled in after `Snapshot(w)` reads the workflow's final state.
- **Replay loses DAG edges**: `ReplayEvents` reconstructs a report from a flat event stream. Dependencies, `MaxAttempts`, `HasRetry`, and `HasTimeout` are not available from events alone — use a Snapshot-captured report for full fidelity.
- **go-workflow retry data race**: `DefaultRetryOption.Backoff` shares a single `ExponentialBackOff` instance that races under concurrent use. This is a known upstream issue in v0.1.13.
- **`WorkflowID` cannot contain path separators** (`/` or `\`): it is used in export filenames and would break path safety. Use `Config.RunID` or your own export paths for arbitrary identifiers.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing patterns, code style, and the release process.

- [Changelog](CHANGELOG.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Report a bug or request a feature](https://github.com/LarsArtmann/go-workflow-auditlog/issues)

## License

[MIT](LICENSE) — Copyright (c) 2026 Lars Artmann
