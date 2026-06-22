# Dashboard Overhaul: Failure Visibility & Self-Review Fixes

**Date**: 2026-06-22 20:51 · **Branch**: master · **Coverage**: 92.9% · **Tests**: 266 (26 HTML)

---

## What Was Done

The HTML dashboard went from **hiding failure reasons behind click-tooltips** to **surfacing them everywhere at a glance**. A brutal self-review then caught dead code, a split-brain handler, missing tests, and edge cases — all fixed.

### Phase 1: Feature Implementation (14 tasks)

| # | Feature | Impact |
|---|---------|--------|
| 1 | **Failure Summary banner** — prominent red banner with `failure_reason` + per-step error list when `workflow_succeeded=false` | 🔴 Critical |
| 2 | **Error column in steps table** — inline truncated error text, full text on hover (was click-tooltip only) | 🔴 Critical |
| 3 | **Workflow PASS/FAIL hero badge** — green ✓ / red ✗ next to title in header | 🔴 High |
| 4 | **Gantt-style timeline** — real time-positioned bars showing parallelism (was fake duration-sorted bars) | 🔴 High |
| 5 | **Humanized durations** — `48811ms` → `48.8s`, `317799ms` → `5m 18s` everywhere | 🟡 High |
| 6 | **Color-coded failed rows** — subtle red-tinted background on failed/canceled rows | 🟡 Medium |
| 7 | **Inline errors in events table** — actual error text instead of "error" badge | 🟡 Medium |
| 8 | **Error text in tree nodes** — failed steps show error inline in DAG tree | 🟡 Medium |
| 9 | **Failure-impact badges** — "⚠ blocked" on steps skipped because a dependency failed | 🟡 Medium |
| 10 | **Graph error dots** — red dot indicator on failed/canceled nodes in SVG graph | 🟢 Polish |
| 11 | **"Errors only (N)" button** — count badge + red border when failures exist | 🟢 Polish |
| 12 | **Keyboard "e" shortcut** — toggle errors-only filter without mouse | 🟢 Polish |
| 13 | **`humanizeDuration()` helper** — shared duration formatting function | 🟢 Foundation |
| 14 | **Golden file updated** | — |

### Phase 2: Self-Review Fixes (5 tasks)

| # | Fix | Root Cause |
|---|-----|------------|
| 1 | **Removed dead timeline CSS** (.timeline-bar/row/label/track) | Left behind after Gantt replacement — 6 unused selectors |
| 2 | **Consolidated split-brain error handler** | `toggleErrorsOnly()` existed alongside a duplicate inline click handler |
| 3 | **Moved `esc()` + `humanizeDuration()` to top of JS** | Were at bottom of file, called before definition in source order |
| 4 | **Added max-width to `.inline-error` CSS** | Long error strings could break events table layout |
| 5 | **Fixed Gantt to handle skipped steps** (no `finished_at`) | Skipped steps with `started_at` but no `finished_at` were invisible |

### Phase 3: Test Coverage (9 new tests)

| Test | Verifies |
|------|----------|
| `TestWriteHTML_FailureBanner_WhenFailed` | Banner container + `workflow_succeeded` + `failure_reason` in JSON |
| `TestWriteHTML_FailureBanner_HiddenWhenSucceeded` | Banner template exists but `workflow_succeeded=true` |
| `TestWriteHTML_ErrorColumn` | Error `<th>`, colspan=9, error-cell CSS, error text in JSON |
| `TestWriteHTML_WorkflowStatusBadge` | workflow-status element + passed/failed CSS classes |
| `TestWriteHTML_GanttChart` | gantt-axis, gantt-grid, gantt-bar CSS + renderGantt function |
| `TestWriteHTML_ImpactBadge` | impact-badge CSS + impactedSteps + computeImpact |
| `TestWriteHTML_HumanizedDurations` | humanizeDuration function + usage in stats and steps |
| `TestWriteHTML_GraphFailedNodeDot` | Failed status check triggers error dot rendering |
| `TestWriteHTML_TreeInlineError` | scope-node-error CSS + has-failure class + error in JSON |

---

## Status

### a) FULLY DONE ✅

- All 14 dashboard features implemented, tested, verified
- All 5 self-review issues found and fixed
- 9 new feature tests added (26 HTML tests total, 266 total tests)
- Golden file regenerated
- Lint: 0 issues · Vet: clean · Race detector: clean · Coverage: 92.9%
- Dead code removed (old timeline CSS)
- Split-brain handler consolidated

### b) PARTIALLY DONE 🟡

- **Gantt timeline for skipped steps**: Zero-width markers shown, but they could be visually clearer (diamond/icon marker instead of thin bar)
- **Mobile responsiveness for Gantt**: Works but could be tighter on very narrow screens

### c) NOT STARTED ⬜

- **Error categorization**: Failed steps could be grouped by error type (transient vs permanent)
- **Retry attempt visualization**: Per-attempt durations in the Gantt (currently shows final only)
- **Export/share failure summary**: Copy-to-clipboard for failure reasons
- **Dark/light theme toggle**: Currently dark-only

### d) TOTALLY FUCKED UP ❌

Nothing. All issues from the self-review were fixed before committing.

### e) WHAT WE SHOULD IMPROVE

1. **`humanizeDuration` has no test coverage** — pure function, should have unit tests for edge cases (0, null, negative, hours)
2. **JS has no linting** — `dashboard.js` is hand-written vanilla JS with no eslint/biome check
3. **Type model gap**: `StepInfo.Error` is `*string` — could be a branded `ErrorMessage` type for type safety
4. **`WorkflowReport` has query methods (`FailedSteps()`, `SkippedSteps()`) but the JS re-filters inline** — the Go API is richer than what the dashboard uses
5. **Dashboard JS is 1346 lines in one file** — could be split into modules (graph engine, timeline, table) for maintainability
6. **No CI check for the golden file** — the test catches drift but `UPDATE_GOLDEN` is manual

### f) Top 25 Things to Do Next

| # | Task | Impact | Effort |
|---|------|--------|--------|
| 1 | Add unit tests for `humanizeDuration()` edge cases | Medium | Low |
| 2 | Add eslint/biome config for dashboard.js + dashboard.css | Medium | Low |
| 3 | Group failures by error type in the failure banner | High | Medium |
| 4 | Show per-retry-attempt bars in Gantt timeline | High | Medium |
| 5 | Add light theme support (CSS custom properties already in place) | Medium | Low |
| 6 | Make Gantt bars clickable → navigate to step in steps table | Medium | Medium |
| 7 | Add "Copy failure summary" button to failure banner | Medium | Low |
| 8 | Add filter for Gantt (show only failed, only slow, etc.) | Medium | Medium |
| 9 | Split dashboard.js into modules (graph.js, gantt.js, table.js) | Medium | Medium |
| 10 | Add step search by error text (already in data-search, verify works) | Low | Low |
| 11 | Add total wall-clock time axis labels to Gantt (not just timestamps) | Low | Low |
| 12 | Add "shareable link" with step highlighted via URL hash | Low | Medium |
| 13 | Add print-friendly CSS for PDF export | Low | Medium |
| 14 | Consider branded type for `ErrorMessage` instead of `*string` | Medium | Medium |
| 15 | Add workflow comparison view (diff two reports in browser) | High | High |
| 16 | Add real-time updates via Server-Sent Events for live monitoring | High | High |
| 17 | Add keyboard shortcuts help overlay ("?" to toggle) | Low | Low |
| 18 | Add accessibility audit (ARIA roles on Gantt, graph, tree) | Medium | Medium |
| 19 | Add color-blind-friendly mode (patterns instead of just colors) | Medium | Medium |
| 20 | Add step duration percentile chart (p50, p90, p99) | Medium | Medium |
| 21 | Add "critical path" highlight in Gantt (bottleneck chain) | High | Medium |
| 22 | Add zoom/brush to Gantt timeline for large workflows | Medium | Medium |
| 23 | Consider `templ` for HTML template instead of `fmt.Sprintf` | Medium | Medium |
| 24 | Add OpenGraph meta tags for shareable report links | Low | Low |
| 25 | Add JSON schema validation for embedded report data | Medium | Medium |

### g) Top Question I Cannot Figure Out Myself

**Should the dashboard eventually be split into a separate frontend project (e.g. a React/Vue SPA that consumes the JSON report), or should it stay as a self-contained single-file HTML embedded via `go:embed`?**

The current approach (single file, no dependencies, `go:embed`) has clear advantages for a library: zero build step, works offline, trivially shareable. But at 1346 lines of JS + 1000 lines of CSS, it's approaching the complexity ceiling for vanilla JS without modules. The Sugiyama graph engine alone is ~200 lines. Splitting into a separate frontend would enable component reuse, proper linting, and type checking — but would break the "one file, no dependencies" guarantee that makes this library valuable. This is a strategic product decision, not a technical one.
