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
