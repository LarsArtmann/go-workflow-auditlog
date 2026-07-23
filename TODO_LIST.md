# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md).

---

## Testing

- [ ] Add JS runtime test coverage for dashboard functions (enhanceGraph, computeCriticalPathSteps, applyGraphSearch) — consider headless browser (Playwright) or JS unit test runner

## Visualization

- [ ] Highlight critical path by default when graph tab opens (if path has >1 step)
- [ ] Add minimap for large graphs (>20 nodes)
- [ ] Add graph layout direction toggle (TD/LR) matching diagram export options
- [ ] Add "fit to view" on initial graph render

---

## Deferred (needs consumer demand or architectural decision — see ROADMAP.md)

- [ ] OpenTelemetry span bridge (`attempt_start` → span start, `attempt_end` → span end)
- [ ] CLI tool (`auditlog`) for inspecting/replaying/diffing exported reports
- [ ] Time-based streaming flush (`WithFlushInterval(d time.Duration)`)
- [ ] Async channel-based streaming writer (backpressure decoupling)
- [ ] `MultiWriter` fan-out to multiple `OnEvent` callbacks
- [ ] `Diff()` on PeakConcurrency / CriticalPath (currently only duration delta)
- [ ] `FailureReason` structured categories (typed enum, not just a string)
