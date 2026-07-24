# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md).

---

## Infrastructure (blocking)

(none currently)

## Live Dashboard (`live/` module)

- [ ] Live DAG graph during execution — needs DAG structure available before `Do()`; currently only renders on `snapshot` or `complete` events
- [ ] Optimize steps table rendering — rebuilds entirely on each render tick (flicker for 100+ steps); consider diff-based DOM updates
- [ ] Add concurrent Subscribe/Unsubscribe test for Hub
- [ ] Add SSE reconnect/heartbeat test

## Testing

- [ ] Add JS runtime test coverage for dashboard functions (`enhanceGraph`, `computeCriticalPathSteps`, `applyGraphSearch`) — consider headless browser (Playwright) or JS unit test runner
- [ ] Add fuzz test for `NDJSONStreamer` (streaming encode/flush error paths)
- [ ] Add streaming round-trip property test (streamed events == batch events)

## Visualization

- [ ] Highlight critical path by default when graph tab opens (if path has >1 step)
- [ ] Add minimap for large graphs (>20 nodes)
- [ ] Add graph layout direction toggle (TD/LR) matching diagram export options
- [ ] Add "fit to view" on initial graph render
- [ ] Add `--output-dir` flag to `viz/example/main.go` (root cause of repeated file-clobbering incidents)

## Polish

- [ ] Migrate benchmarks from `b.N` to `b.Loop()` (gopls `stdversion` warnings)
- [ ] Add CONTRIBUTING.md
