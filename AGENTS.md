# AGENTS.md — go-workflow-auditlog

Go library for [Azure/go-workflow](https://github.com/Azure/go-workflow) that records every step execution event (attempts, retries, durations, errors, dependencies, final statuses) with timestamps and export to JSON / NDJSON.

**Module**: `github.com/larsartmann/go-workflow-auditlog` · **Package**: `auditlog` · **Go**: 1.26+ · **Status**: ALPHA

---

## Commands

| Command                                                         | Purpose                           |
| --------------------------------------------------------------- | --------------------------------- |
| `go test ./...`                                                 | Run all tests                     |
| `go test -race ./...`                                           | Run all tests with race detector  |
| `go test -race -coverprofile=cover.out -covermode=atomic ./...` | Tests with coverage (~95%)        |
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
step.go            — StepInfo type (embeds StepRef, carries StepID) + methods (HasError, DeriveStatus)
plugin.go          — Public API: New(), Attach(), Snapshot(), Report(), Export*(), Config, Auditor, RunID()
recorder.go        — Core state machine: event capture, step records, attempt tracking, RunID + StepID counters
runid.go           — newRunID(): 128-bit crypto-random hex run identifier
attach.go          — Attach/Snapshot logic: callback injection + post-run DAG capture (incl. sub-workflows)
report.go          — WorkflowReport type (carries RunID) + Validate() + sentinel errors + query methods + WriteJSON
report_builder.go  — BuildReport assembly: step records → sorted StepInfo + aggregates (WorkflowSucceeded)
filter.go          — Report filtering (Filtered, ReportOption, WithStepsByStatus, etc.)
diff.go            — Diff API: DiffResult/StepDiff, Diff() between reports, Duration() wall-clock, Summary()
index.go           — ReportIndex: opt-in O(1) lookup maps over a report
loader.go          — LoadReport / LoadReportFromReader / LoadReportFromBytes
export.go          — NDJSON writer (writeEventsNDJSON internal helper)
ndjson.go          — ReadEvents NDJSON reader (sentinel errors)
replay.go          — ReplayEvents: reconstruct Report from event stream (preserves RunID + assigns StepIDs)
diagram.go         — Translation layer: buildGraph() converts WorkflowReport → go-output GraphNode/GraphEdge + statusStyle() maps StepStatus → GraphStyle colors
mermaid.go         — Mermaid flowchart export (delegates to go-output graph.MermaidRenderer, code fence off)
plantuml.go        — PlantUML component diagram export (delegates to go-output plantuml.PlantUMLDiagram)
graphviz.go        — Graphviz DOT digraph export (delegates to go-output graph.DOTRenderer, graphID "workflow")
example/           — Data pipeline demo (fetch → validate → transform → save + retry); version ldflags
.goreleaser.yml    — Automated GitHub releases with grouped changelog + demo binary
```

### Data Flow

1. User creates `Auditor` via `New(Config)`
2. `Attach(w)` injects `BeforeStep`/`AfterStep` callbacks into all steps via `State.MergeConfig`
3. During `w.Do(ctx)`, callbacks fire per-attempt → `Recorder` captures timestamped `Event`s
4. `Snapshot(w)` reads `w.StateOf(step)` + `w.UpstreamOf(step)` to fill in DAG structure and skipped/canceled statuses
5. `Report()` assembles `StepInfo` slice (with forward + reverse deps) and event stream
6. Export methods serialize to JSON, NDJSON, or diagrams (Mermaid/PlantUML/DOT via [go-output](https://github.com/larsartmann/go-output))

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
- **Diagram export** uses [go-output](https://github.com/larsartmann/go-output) renderers. `buildGraph()` in `diagram.go` translates `WorkflowReport` steps into `output.GraphNode` + `output.GraphEdge` (edges point step → dependency). `statusStyle()` maps `StepStatus` → `output.GraphStyle` fill colors (`succeeded`=`#2d5a2d` green, `failed`=`#8b2d2d` red, `skipped`=`#4a4a4a` gray, `canceled`=`#5a3d2d` orange). Mermaid uses per-node `style` directives (not `classDef`); code fence is OFF for raw `.mmd` output. DOT uses `graphID "workflow"`. PlantUML uses `[label] as id` component notation. Each renderer handles its own ID sanitization and label escaping — auditlog passes raw step names as node IDs. Dependency: `go-output` root v0.17.0 + `graph` v0.13.0 + `plantuml` v0.13.0 (multi-module; sub-modules tagged at v0.13.0, root at v0.17.0; MVS resolves correctly since replace directives in published go.mods are ignored by consumers).
- **`StepInfo.Error` reflects the FINAL outcome only.** For a step that fails on attempts 1–2 and succeeds on attempt 3, the `Error` field is `nil` (not "transient failure" from the last failed attempt). The per-attempt error history is preserved in the `Event` stream — each `attempt_end` event carries its own `Error`. Rationale: the step-level `Error` is the answer to "why did this step end in its final state?", and a succeeded step ended successfully. See `recorder.go:recordAfterStep` and the regression test `TestRetry_StepErrorClearedOnSuccess`.

---

## Testing Patterns

- Standard `testing.T` + table-driven tests. No ginkgo/testify.
- Test steps implement `String()` for deterministic names.
- Retry tests use fresh `backoff.NewExponentialBackOff()` to avoid the go-workflow race.
- External test package (`auditlog_test`).
- `t.Setenv()` for env var tests (runs sequentially).
- 125+ tests covering: disabled/enabled, success/failure, dependencies, retry, timeout/cancel, skip, concurrent steps, fan-out/fan-in, event ordering, OnEvent callback, export formats (JSON/NDJSON), report validation, query methods, filter, diff, replay, load, diagrams (Mermaid/PlantUML), edge cases, plus regression tests for fixed bugs (status drift, diff ordering, NDJSON line numbers, WorkflowSucceeded honesty about pending steps, stale error cleared on retry success).
- Coverage: **94.0%** of statements (auditlog package).

### Shared test helpers (in `auditlog_test.go`)

| Helper                           | Purpose                                                       |
| -------------------------------- | ------------------------------------------------------------- |
| `mustNew`, `newAuditAndWorkflow` | Construct auditor + workflow fixtures                         |
| `retryOpts`, `addRetryStep`      | Wrap a step with retry config (fresh backoff)                 |
| `addParallelSteps`               | Wire two independent steps (no dependency edge)               |
| `addDependentStep`               | Wire a parent→child dependency chain                          |
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
