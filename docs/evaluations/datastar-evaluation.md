# Evaluation: Datastar for go-workflow-auditlog Live Dashboard

**Date:** 2026-08-07
**Status:** Decision document — **ADOPT** (medium priority)

---

## Context

The live dashboard (`live/dashboard.js`) is a 1,922-line hand-written JavaScript
IIFE with no dependencies. It handles SSE parsing, state management, DOM
rendering (10+ render functions), incremental diff-based updates, SVG graph
manipulation, keyboard accessibility, and reconnection logic.

The sibling project `samber-do-auditlog` already adopted [Datastar](https://data-star.dev)
(v1.0.2, ~56KB embedded runtime) and reduced its dashboard JS to ~50 lines of
inline keyboard shortcuts and export helpers.

---

## Current State

| Aspect | Detail |
|--------|--------|
| File size | 1,922 lines (`live/dashboard.js`) |
| Dependencies | Zero (vanilla JS IIFE) |
| SSE parsing | Manual `EventSource` + 3 named events + exponential backoff |
| State management | Mutable `state` object with `report`, `events[]`, `steps{}`, `dag` |
| Rendering | 10+ functions building HTML via string concatenation with manual `esc()` |
| Incremental updates | Hand-rolled `stepStateKey()` fingerprinting + `updateStepRow()` cell patching |
| Graph | Direct SVG manipulation, `MutationObserver` for minimap |
| Keyboard nav | ~120 lines: roving tabindex, focus traps, arrow-key navigation |

---

## What Datastar Eliminates

Adopting Datastar would eliminate **~1,500+ lines** of dashboard JS:

- **SSE parsing** — Datastar handles SSE transport and `datastar-patch-elements` events natively
- **State management** — Datastar signals replace the manual `state` object
- **DOM rendering** — All 10 render functions replaced by server-side templ fragments
- **Incremental diffing** — Datastar's DOM morphing handles incremental updates by element ID
- **HTML escaping** — Server-side templ handles escaping; no manual `esc()` needed
- **Reconnection** — Datastar handles SSE reconnection automatically

**What survives** (~100-200 lines):
- SVG graph enhancement (`enhanceGraph`, critical-path highlighting, retry badges)
- Minimap viewport tracking via `MutationObserver`
- Keyboard accessibility (focus traps, roving tabindex)
- Export button helpers

---

## Migration Path

### Phase 1: Server-side fragment rendering
1. Add `github.com/a-h/templ` dependency (already available — samber-do uses it)
2. Create `live/fragments.templ` with reactive components for stats, steps table, events table, timeline, waveform
3. Create `live/fragments.go` with Go helpers (humanizeMs, status badges, etc.)
4. Rewrite SSE handler to emit `datastar-patch-elements` events instead of JSON

### Phase 2: Dashboard skeleton
1. Embed `datastar.js` via `go:embed`
2. Rewrite `live/dashboard.go` with Datastar directives (`data-signals`, `data-text`, `data-show`, `data-on:click`)
3. Remove `live/dashboard.js` — replace with ~50 lines of inline JS for keyboard nav and graph enhancement

### Phase 3: Graph enhancement preservation
1. Port SVG manipulation code into a small inline `<script>` block
2. Keep keyboard accessibility as inline JS

---

## Risks

| Risk | Mitigation |
|------|------------|
| Datastar bundle size (~56KB) | Acceptable — embedded via `go:embed`, served gzipped |
| Learning curve for Datastar directives | Reference samber-do's working implementation |
| SVG graph manipulation doesn't map to Datastar | Keep as inline JS (same pattern as samber-do) |
| Keyboard accessibility needs custom JS | Already the pattern in samber-do (~50 lines inline) |

---

## Recommendation

**ADOPT Datastar** when next touching the live dashboard module. The migration
eliminates ~80% of the dashboard JS, follows a proven pattern from the sibling
project, and aligns the two projects' architectures.

**Priority:** Medium. The current dashboard works, but is a maintenance burden
(1,922 lines of JS in a Go library). Schedule for after the v0.9.0 release.

---

## References

- samber-do-auditlog `live/dashboard.go` — Datastar dashboard skeleton
- samber-do-auditlog `live/fragments.templ` — Server-side fragment templates
- samber-do-auditlog `live/fragments.go` — Go helpers for fragment rendering
- [Datastar documentation](https://data-star.dev)
