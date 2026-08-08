# Evaluation: templ for go-workflow-auditlog HTML Rendering

**Date:** 2026-08-07
**Status:** Decision document — **DEFER** (adopt with Datastar, not standalone)

---

## Context

The project generates self-contained HTML reports via `viz/html_render.go`.
The sibling project `samber-do-auditlog` uses `github.com/a-h/templ` for
type-safe Go templates in both static HTML and live dashboard rendering.

This evaluation assesses whether adopting templ would improve the current
HTML rendering pipeline.

---

## Current State

| Aspect | Detail |
|--------|--------|
| File | `viz/html_render.go` (184 lines) |
| Template | Single `const htmlTemplate` string literal (118 lines) |
| Dynamic data | 3 JSON blobs injected via `<script type="application/json">` tags |
| Substitution | 8 `fmt.Sprintf` verbs in the template string |
| XSS safety | All dynamic data is JSON in script tags (no string-interpolated HTML) |
| Conditional logic | None — the template is fully static |
| Loops | None — all iteration happens client-side in dashboard.js |

---

## Analysis

### What templ would replace

The current `htmlTemplate` is a static HTML skeleton:

```go
const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
  <title>%s</title>
  <style>%s</style>
</head>
<body>
  <div id="dashboard"></div>
  <script type="application/json" id="report-data">%s</script>
  <script type="application/json" id="metadata">%s</script>
  <script type="application/json" id="dag-data">%s</script>
  <script>%s</script>
</body>
</html>`
```

templ would make this:

```templ
templ Dashboard(title string, css string, report json.RawMessage, ...) {
    <!DOCTYPE html>
    <html>
    <head><title>{ title }</title>...</head>
    <body>...</body>
    </html>
}
```

### Benefit assessment

| Criterion | Current (fmt.Sprintf) | templ | Verdict |
|-----------|----------------------|-------|---------|
| Type safety | None (8 untyped `%s`) | Full (typed params) | Marginal — only 8 params |
| XSS safety | Already safe (JSON in script tags) | Also safe | No improvement |
| Conditional rendering | Not needed | Available | No current use case |
| Loops over data | Not needed (client-side) | Available | No current use case |
| Build step | None | `go tool templ generate` | **Added complexity** |
| Generated code | None | `html_templ.go` | **Extra maintenance** |

### When templ becomes valuable

templ becomes valuable when combined with **Datastar** (see
[datastar-evaluation.md](datastar-evaluation.md)):

1. **Server-side fragment rendering** — templ renders reactive HTML fragments
   (table rows, waveform bars, stats cards) that Datastar morphs into the DOM
2. **Live dashboard HTML** — templ provides the dashboard skeleton with
   Datastar directives, replacing string concatenation
3. **Type-safe data binding** — templ components receive typed Go structs,
   eliminating the JSON-in-script-tag indirection for the live dashboard

Without Datastar, the static HTML report template is too simple to justify
the templ build step.

---

## Recommendation

**DEFER templ adoption** until the Datastar migration happens. At that point,
adopt templ together with Datastar to render reactive fragments server-side
(the `live/fragments.templ` pattern from samber-do-auditlog).

**Do NOT adopt templ solely for `viz/html_render.go`** — the current static
template is safe, simple, and doesn't benefit from type-safe templating.

---

## References

- samber-do-auditlog `html.templ` — Static HTML report template (templ)
- samber-do-auditlog `live/fragments.templ` — Reactive fragment templates (templ + Datastar)
- [templ documentation](https://templ.guide)
- [datastar-evaluation.md](datastar-evaluation.md) — Companion evaluation
