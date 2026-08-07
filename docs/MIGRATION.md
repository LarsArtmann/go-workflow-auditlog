# Migration Guide

## Module Split (v0.8.0)

The library is split into two independent Go modules. Consumers who only need
JSON/NDJSON audit trails can import the core module with zero go-output
dependency cost. Visualization consumers add the `viz` sub-module.

### Before (single module)

```bash
go get github.com/larsartmann/go-workflow-auditlog
```

```go
import (
    auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// Everything lived in one module — including visualization functions.
```

### After (two modules)

```bash
# Core only (event capture, JSON/NDJSON, replay, diff, filter, index)
go get github.com/larsartmann/go-workflow-auditlog

# Add visualization (diagrams, tables, trees, HTML dashboard)
go get github.com/larsartmann/go-workflow-auditlog/viz
```

```go
import (
    auditlog "github.com/larsartmann/go-workflow-auditlog"
    viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

// Core: JSON/NDJSON export, replay, diff, filter, index
_ = audit.ExportJSON("report.json")

// Visualization: diagrams, tables, trees, HTML dashboard
_ = viz.ExportHTML(report, "dashboard.html")
_ = viz.ExportMermaid(report, "dag.mmd")
```

### What changed

| Aspect            | Before                                                          | After                                                               |
| ----------------- | --------------------------------------------------------------- | ------------------------------------------------------------------- |
| Viz import path   | `github.com/larsartmann/go-workflow-auditlog/viz` (same module) | `github.com/larsartmann/go-workflow-auditlog/viz` (separate module) |
| Viz `go get`      | Included in core                                                | Separate `go get github.com/larsartmann/go-workflow-auditlog/viz`   |
| Core dependencies | Included go-output transitively                                 | 3 direct deps, zero go-output                                       |
| `go.work`         | Single module                                                   | Workspace linking core + viz                                        |

### Migration steps

1. **If you only use JSON/NDJSON**: No changes needed. The core import path is
   unchanged. You automatically shed the go-output dependency.

2. **If you use diagrams/tables/trees/HTML**: Add the viz module to your
   `go.mod`:

   ```bash
   go get github.com/larsartmann/go-workflow-auditlog/viz
   ```

   Your import statements stay the same (`viz` package path is unchanged).

3. **If you import `testhelpers`**: No changes needed. The `testhelpers`
   package lives inside the core module so both core and viz tests can import
   it without a circular module dependency.

### Removed API aliases (v0.5.1)

These backward-compat aliases were removed in v0.5.1 (never in a released v1.0):

| Old name               | New name       |
| ---------------------- | -------------- |
| `WriteReportJSON`      | `WriteJSON`    |
| `WriteEventsNDJSON`    | `WriteNDJSON`  |
| `ExportToFile`         | `ExportJSON`   |
| `ExportEventsToNDJSON` | `ExportNDJSON` |

## WebSocket Transport Removal (v0.8.3)

The `/api/ws` WebSocket endpoint and its `gorilla/websocket` dependency have
been removed from the `live` module. The dashboard now uses SSE exclusively.

### Migration

**No client-side changes are required.** All browsers support SSE natively via
the `EventSource` API, and the live module's reconnection replay
(via `Last-Event-ID`) covers any transient disconnects — including scenarios
behind corporate proxies that originally motivated the WebSocket fallback.

**If you wrote custom code that dialed `/api/ws` directly** (rare; the
dashboard JS is the only intended consumer), switch to `EventSource` against
`/api/events`:

```js
// Before (no longer supported)
const ws = new WebSocket(`ws://${host}/api/ws`);
ws.onmessage = (e) => {
	handle(JSON.parse(e.data));
};

// After
const es = new EventSource(`http://${host}/api/events`);
es.addEventListener("snapshot", (e) => handle(JSON.parse(e.data)));
es.addEventListener("event", (e) => handle(JSON.parse(e.data)));
es.addEventListener("complete", (e) => handle(JSON.parse(e.data)));
```

**If your `go.mod` references `gorilla/websocket` solely for this transport**,
you can drop the dependency. The `live` module no longer requires it.

### Why

The WebSocket transport was an additive convenience, never required: SSE
serves every browser without a framing protocol, with native reconnection
support, and with `Last-Event-ID` replay now (v0.8.x) preserving events
across brief disconnects. The parallel transport duplicated snapshot/event/
complete envelope logic, lacked replay semantics, and required a heavyweight
dependency (`gorilla/websocket` v1.5.3) for no concrete customer benefit.
The sibling [`samber-do-auditlog/live`](https://github.com/larsartmann/samber-do-auditlog)
module has always been SSE-only; this aligns the two siblings on the same
architecture.

## Report `failure_reason` → `failure_summary` (Unreleased)

The `WorkflowReport` JSON field `"failure_reason"` has been renamed to
`"failure_summary"`. This resolves a naming collision with the new
`Event.FailureReason` enum (JSON: `"failure_reason"`), which carries a
structured category (`timeout`, `canceled`, `user_error`) rather than a
human-readable sentence.

### What changed

| Scope            | JSON key before  | JSON key after               | Type                 | Example value                          |
| ---------------- | ---------------- | ---------------------------- | -------------------- | -------------------------------------- |
| `WorkflowReport` | `failure_reason` | `failure_summary`            | `string`             | `"3 step(s) failed: fetch, transform"` |
| `Event`          | `failure_reason` | `failure_reason` (unchanged) | `FailureReason` enum | `"timeout"`                            |

### Migration

1. **If you parse report JSON**: rename your struct field or JSON tag from
   `failure_reason` to `failure_summary`.

   ```go
   // Before
   type MyReport struct {
       FailureReason string `json:"failure_reason"`
   }

   // After
   type MyReport struct {
       FailureSummary string `json:"failure_summary"`
   }
   ```

2. **If you parse event NDJSON**: no changes needed — the event-level
   `"failure_reason"` key is unchanged.

3. **If you use the Go API directly**: rename field accesses from
   `report.FailureReason` to `report.FailureSummary`.

### Why

The report-level field is a human-readable summary, not a structured reason.
The new `Event.FailureReason` typed enum owns the `"failure_reason"` JSON key
with a clear semantic contract (three machine-readable values). Reusing the
same key name for a free-form sentence on a different type created ambiguity
for consumers parsing both events and reports.

## `StepInfo.FailureReason` additive field (Unreleased)

The `StepInfo` struct now carries a `FailureReason` field (`json:"failure_reason,omitempty"`),
denormalized from the last `attempt_end` event. This means consumers no longer
need to scan the event stream to determine why a step failed — the structured
enum (`timeout`, `canceled`, `user_error`) is available directly on the step.

### What changed

| Scope      | JSON key           | Type                 | Example value | Default (success) |
| ---------- | ------------------ | -------------------- | ------------- | ----------------- |
| `StepInfo` | `failure_reason`   | `FailureReason` enum | `"timeout"`   | omitted (`omitempty`) |

### Migration

**No changes required.** The field is additive (`omitempty`), so existing JSON
parsing continues to work. Consumers that want structured failure classification
on steps can now read `step.FailureReason` directly instead of scanning events.

The field reflects the **final outcome only** — for a step that fails on
attempts 1-2 and succeeds on attempt 3, `FailureReason` is empty (the step
succeeded). Per-attempt failure reasons are preserved in the event stream.
