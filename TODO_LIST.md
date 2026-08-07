# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks, verified against the actual code on 2026-08-06.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md) — never retained here.

---

## Release

- [ ] **Cut v0.9.0** — coordinated three-module release resolving the `nix run .#check` standalone failure (core `FailureSummary` rename breaks `viz` standalone builds until published). Read [`RELEASE.md`](RELEASE.md) first. Three annotated tags at one commit: `v0.9.0`, `viz/v0.9.0`, `live/v0.9.0`. Pre-release: `grep -r '^replace' viz/go.mod live/go.mod` must return nothing; working tree must be clean.
  _Source: `docs/status/2026-08-06_23-56` §b.3, `docs/status/2026-08-06_23-23` §d.1_

---

## Infrastructure

- [ ] **Consider Go 1.26.5 → 1.27 upgrade** — eliminates the `GOEXPERIMENT=jsonv2` experimental flag requirement and the 29 gopls `stdversion` warnings across all files using `encoding/json/v2`.
  _Source: `docs/status/2026-08-06_20-14` §f.36-37_

---

## Deferred (design decision)

- [ ] **Full browser automation tests (Playwright or chromedp)** for the live dashboard — **deferred by design**. `go-workflow-auditlog` is a Go _library_, not an application. Adding a browser-binary runtime dependency (Chromium, ~300 MB) to the default `go test` / `nix run .#check` would make CI heavy, slow, platform-dependent, and flaky — a net negative for a library's CI. The existing Go-based JS structural tests already cover function presence, event wiring, diff-based rendering infrastructure, SSE connection logic, and CSS integrity. If pixel-level click→render→DOM flows become necessary, gate them behind a `//go:build browser_e2e` tag (excluded from default CI) and run them only in a dedicated environment with Chromium provisioned. Tracked as a candidate in [ROADMAP.md](./ROADMAP.md).
