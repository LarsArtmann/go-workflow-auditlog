# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks, verified against the actual code.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md) — never retained here.

---

## Deferred (design decision)

- [ ] **Full browser automation tests (Playwright or chromedp)** for the live
      dashboard — **deferred by design**. `go-workflow-auditlog` is a Go _library_,
      not an application. Adding a browser-binary runtime dependency (Chromium,
      ~300 MB) to the default `go test` / `nix run .#check` would make CI heavy,
      slow, platform-dependent, and flaky — a net negative for a library's CI. The
      existing Go-based JS structural tests already cover function presence, event
      wiring, diff-based rendering infrastructure, SSE connection logic, and CSS
      integrity. If pixel-level click→render→DOM flows become necessary, gate them
      behind a `//go:build browser_e2e` tag (excluded from default CI) and run them
      only in a dedicated environment with Chromium provisioned. Tracked as a
      candidate in [ROADMAP.md](./ROADMAP.md).
