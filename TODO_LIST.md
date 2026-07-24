# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md).

---

## Live Dashboard (`live/` module)

- [ ] Add SSE end-to-end integration test (real workflow → live server → SSE client → verify events match)
- [ ] Add WebSocket transport alternative to SSE (for environments that block SSE)

## Testing

- [ ] Add Playwright-based browser tests for live dashboard interactions (graph click navigation, search filtering, tab switching)
- [ ] Improve live module test coverage to 95%+ (currently 90.4%, gap is mostly in error paths within handleSSE/sendSnapshot)

## Visualization

- [ ] Add "export dashboard" button to live view (snapshot current state to standalone HTML)
- [ ] Add step duration labels to graph nodes in live mode (matching static dashboard)
