# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md).

---

## Live Dashboard (`live/` module)

- [ ] Improve live module test coverage to 95%+ (currently 90%, gap is SSE error paths: heartbeat timing, WriteEvent failures, context cancellation edge cases)

## Testing

- [ ] Add full browser automation tests (Playwright or chromedp) for live dashboard pixel-level interactions — current Go-based structural tests validate function presence, event wiring, diff-based rendering infrastructure, WebSocket fallback, and CSS integrity, but cannot test click→render→DOM-update flows at the pixel level
