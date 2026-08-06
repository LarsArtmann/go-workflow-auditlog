# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks, verified against the actual code on 2026-08-06.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md) — never retained here.

---

## Release

- [ ] **Cut v0.9.0** — coordinated three-module release resolving the `nix run .#check` standalone failure (core `FailureSummary` rename breaks `viz` standalone builds until published). Read [`RELEASE.md`](RELEASE.md) first. Three annotated tags at one commit: `v0.9.0`, `viz/v0.9.0`, `live/v0.9.0`. Pre-release: `grep -r '^replace' viz/go.mod live/go.mod` must return nothing; working tree must be clean.
  _Source: `docs/status/2026-08-06_23-56` §b.3, `docs/status/2026-08-06_23-23` §d.1_

---

## Features

- [ ] **Surface `FailureReason` in viz dashboard** — the structured enum (`timeout`/`canceled`/`user_error`) is captured on `Event` but never visualized. Add to steps table, graph node labels, and timeline.
  _Verified: `grep -r 'FailureReason' viz/*.go` returns 0 matches._
  _Source: `docs/status/2026-08-06_23-56` §f.30, `docs/status/2026-08-06_22-27` §f.34-35_

- [ ] **Surface `FailureReason` in CSV/TSV export** — add a `failure_reason` column to the CSV output (`csv.go`). Currently 0 references.
  _Verified: `grep -c 'FailureReason' csv.go` returns 0._
  _Source: `docs/status/2026-08-06_23-56` §f.29_

- [ ] **Denormalize `FailureReason` onto `StepInfo`** — store the last attempt's reason on the step so consumers don't have to scan the event stream. Currently only on `Event`.
  _Source: `docs/status/2026-08-06_23-56` §f.27, `docs/status/2026-08-06_23-23` §e.3_

---

## Coverage & Testing

- [ ] **Close `StreamEvents` coverage gap** (93.9% → ~100%) — add `TestStreamEvents_AllLinesFailJSON` for non-blank input where every line fails JSON parsing (maps to `ErrNoEvents`).
  _Source: `docs/status/2026-08-06_23-56` §b.1_

- [ ] **Close `classifyFailure` coverage gap** (85.7% → ~100%) — add a test running a successful step through the recorder and verifying `attempt_end` event has `FailureReason == ""` (nil-error path).
  _Source: `docs/status/2026-08-06_23-56` §b.2, §e.2_

- [ ] **Add `FailureSummary` golden JSON test** — serialize a `WorkflowReport` with failures, verify `"failure_summary"` appears at report level, `"failure_reason"` does NOT appear at report level (but DOES appear at event level). Catches future field-collision regressions.
  _Source: `docs/status/2026-08-06_23-56` §e.4_

- [ ] **Add `FuzzStreamEvents` fuzz target** — fuzz with arbitrary NDJSON bytes, verify no panic, callback count matches event count or error returned.
  _Source: `docs/status/2026-08-06_23-23` §f.16_

---

## Documentation

- [ ] **Update `README.md`** with new APIs — `MultiWriter`, `StreamEvents`, `FailureReason`/`FailureSummary`, `WithFlushInterval`, and workflow-level helpers (`RetriedStepCount`, `TimedOutSteps`, etc.) are absent from the feature highlights.
  _Verified: `grep -c 'MultiWriter\|StreamEvents\|FailureReason' README.md` returns 0._
  _Source: `docs/status/2026-08-06_23-56` §c.4, `docs/status/2026-08-06_23-23` §f.10_

- [ ] **Update `STABILITY.md`** — document stability promises for new APIs (`StreamEvents`, `MultiWriter`, `FailureReason`, `FailureSummary`, `WithFlushInterval`, workflow helpers). The project is 0.x (alpha) so everything is technically unstable, but explicit documentation helps consumers.
  _Source: `docs/status/2026-08-06_23-56` §c.5, `docs/status/2026-08-06_23-23` §f.11_

- [ ] **Add Architecture Decision Records (ADRs)** for key design decisions: (1) SSE-only transport, (2) `FailureReason` enum with 3 values not 5 (panics/dependencies undetectable at `AfterStep` callback level), (3) `MultiWriter` `func(Event)` signature, (4) `FailureSummary` rename.
  _Verified: no `docs/adr/` directory exists._
  _Source: `docs/status/2026-08-06_23-56` §c.6-13, `docs/status/2026-08-06_23-23` §f.12_

- [ ] **Update `docs/MIGRATION.md`** — document `Event.FailureReason` additive schema addition (new `failure_reason` field on `attempt_end` events, `omitempty`, backward-compatible).
  _Source: `docs/status/2026-08-06_23-56` §c.3, `docs/status/2026-08-06_23-23` §c.7-8_

---

## Infrastructure

- [ ] **Fix pre-commit hook** — the BuildFlow hook (`buildflow --build-mode pre-commit`) is not available in all environments. Every commit this session and prior sessions used `--no-verify`. Either install `buildflow` in `flake.nix` devShell, make the hook resilient to missing binary, or document the workaround.
  _Source: `docs/status/2026-08-06_23-56` §c.1, `docs/status/2026-08-06_23-23` §d.5_

- [ ] **Consider Go 1.26.5 → 1.27 upgrade** — eliminates the `GOEXPERIMENT=jsonv2` experimental flag requirement and the 29 gopls `stdversion` warnings across all files using `encoding/json/v2`.
  _Source: `docs/status/2026-08-06_20-14` §f.36-37_

---

## Deferred (design decision)

- [ ] **Full browser automation tests (Playwright or chromedp)** for the live dashboard — **deferred by design**. `go-workflow-auditlog` is a Go _library_, not an application. Adding a browser-binary runtime dependency (Chromium, ~300 MB) to the default `go test` / `nix run .#check` would make CI heavy, slow, platform-dependent, and flaky — a net negative for a library's CI. The existing Go-based JS structural tests already cover function presence, event wiring, diff-based rendering infrastructure, SSE connection logic, and CSS integrity. If pixel-level click→render→DOM flows become necessary, gate them behind a `//go:build browser_e2e` tag (excluded from default CI) and run them only in a dedicated environment with Chromium provisioned. Tracked as a candidate in [ROADMAP.md](./ROADMAP.md).
