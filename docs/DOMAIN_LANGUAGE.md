# Domain Language

A **Unified Language** for `go-workflow-auditlog` — shared across Customer, Product Owner, Developer, and AI.
Inspired by Domain-Driven Design (DDD) Ubiquitous Language.

Every term below should mean the **same thing** to everyone who reads it.
If a word means something different to a developer than to a customer, define it here.

## Glossary

| Term     | Definition                                                             | Context                            |
| -------- | ---------------------------------------------------------------------- | ---------------------------------- |
| Workflow | A DAG of steps executed by `Azure/go-workflow`.                        | The unit of audit logging.         |
| Auditor  | The `auditlog.Auditor` wrapping a workflow to capture events.          | Public type users interact with.   |
| Step     | A single unit of work implementing `flow.Steper`.                      | Vertices in the DAG.               |
| Attempt  | A single execution of a step (each retry is a new attempt).            | Tracked by `attempt_count`.        |
| Event    | A timestamped observation of an attempt's start or end.                | Serialized as NDJSON; replayable.  |
| Report   | The consolidated, machine-readable snapshot of an audit run.           | Output of `Auditor.Report()`.      |
| Snapshot | The post-execution read of `flow.Workflow` state to capture final DAG. | Captures skipped/canceled steps.   |
| Attach   | Injecting audit callbacks into a workflow's steps before `Do`.         | Must be called before `w.Do(ctx)`. |

## Entities

Objects with identity and lifecycle.

| Term           | Definition                                                                | Context                             |
| -------------- | ------------------------------------------------------------------------- | ----------------------------------- |
| StepInfo       | Aggregated observation for a single step (status, attempts, deps, errors) | Public; serialized in report.Steps  |
| Event          | A single observation (attempt_start / attempt_end)                        | Public; serialized in report.Events |
| WorkflowReport | Denormalized report with aggregates (counts, durations)                   | Public; serialized as JSON          |
| DiffResult     | Difference between two reports (added/removed/changed steps)              | Computed by `WorkflowReport.Diff()` |
| StepDiff       | A single step's state in a diff context                                   | Embedded in DiffResult slices       |

## Value Objects

Immutable objects defined by attributes.

| Term         | Definition                                                          | Context                                                                  |
| ------------ | ------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| StepRef      | `{Name, StepType}` — flattened into Event/StepInfo JSON             | Embedded in Event and StepInfo                                           |
| StepStatus   | Enum: `pending`/`running`/`succeeded`/`failed`/`canceled`/`skipped` | Stable lowercase JSON form, distinct from go-workflow's capitalized form |
| EventType    | Enum: `attempt_start` / `attempt_end`                               | Two-value enum; phase is derived                                         |
| Phase        | Enum: `before` / `after`                                            | Redundant with EventType but explicit in JSON                            |
| ReportOption | Functional-option predicate for filtering reports                   | `WithStepsByName`, `WithStepsByStatus`, etc.                             |
| RunID        | 128-bit hex identifier for one workflow execution                   | Stamped on every Event and WorkflowReport for trace correlation          |
| StepID       | 1-based integer, unique within a run                                | Disambiguates steps that share the same `String()` output                |

## Events

Things that happen in the domain.

| Term          | Definition                                              | Context                               |
| ------------- | ------------------------------------------------------- | ------------------------------------- |
| attempt_start | Fired by `BeforeStep` callback when an attempt begins.  | Recorded with sequence + timestamp    |
| attempt_end   | Fired by `AfterStep` callback when an attempt finishes. | Records duration, error, final status |

## Commands

Actions the system can perform.

| Term                                              | Definition                                           | Context                             |
| ------------------------------------------------- | ---------------------------------------------------- | ----------------------------------- |
| `Attach(w)`                                       | Inject audit callbacks into all workflow steps.      | Before `w.Do(ctx)`.                 |
| `Snapshot(w)`                                     | Read post-execution DAG state into the recorder.     | After `w.Do(ctx)` returns.          |
| `Report()`                                        | Assemble and return the consolidated WorkflowReport. | Read-only; uses RLock.              |
| `Filtered(opts)`                                  | Return a filtered copy of a report.                  | Aggregates recomputed.              |
| `Diff(other)`                                     | Compare two reports.                                 | Returns added/removed/changed.      |
| `ReplayEvents(events)`                            | Reconstruct a report from a flat event stream.       | `Reconstructed=true` on the result. |
| `LoadReport(path)`                                | Read a JSON report from disk.                        | Inverse of `ExportJSON`.          |
| `WriteMermaid(w)` / `PlantUML(w)` / `Graphviz(w)` | Serialize the step DAG as a diagram.                 | For visualization tools.            |

## Bounded Contexts

| Context   | Description                                                               |
| --------- | ------------------------------------------------------------------------- |
| Capture   | Live event recording via callbacks (Recorder, Attach, Snapshot).          |
| Reporting | Building and querying WorkflowReport (BuildReport, query methods).        |
| Export    | Serializing reports/events to JSON, NDJSON, or diagram formats.           |
| Replay    | Reconstructing reports from event streams (ReplayEvents, ReadEvents).     |
| Diff      | Comparing two reports for regression detection (Diff, Duration, Summary). |

---

> **How to use this file:**
>
> - Keep terms concise — one clear sentence per definition
> - Update when new domain concepts emerge
> - Use these terms consistently in code, docs, and conversations
> - When in doubt about a word's meaning, check here first
