# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md).

---

## Documentation

- [ ] Add streaming NDJSON section to `README.md` with usage example (streamer is shipped but undocumented in the end-user guide)
- [ ] Add `CHANGELOG.md` entry for streaming NDJSON feature (missing from `[Unreleased]`)
- [ ] Add streaming NDJSON to `STABILITY.md` API stability matrix
- [ ] Add `NDJSONStreamer` to the export formats table in `FEATURES.md` (currently in prose, not the table)
- [ ] Write consumer migration guide for the module-split API change (old `report.WriteMermaid()` → new `viz.WriteMermaid(report)`)
- [ ] Add package-level `ExampleX` funcs for core-only usage (no viz import)
- [ ] Add package-level `ExampleX` funcs for `viz` functions
- [ ] Create `docs/status/INDEX.md` linking all status reports

## Testing

- [ ] Add core test for `writeEventsNDJSON` flush error path (currently covered only from viz test suite — 80% from core)
- [ ] Add MaxEvents + streaming interaction test (verify streamer sees dropped events)
- [ ] Add fuzz test `FuzzNDJSONStreamer` — adversarial event payloads
- [ ] Add property test for NDJSONStreamer round-trip (stream N events → read → verify)
- [ ] Add `WithBufferSize(0)` and `WithBufferSize(-1)` tests (verify default is kept)
- [ ] Add `ReplayEvents` round-trip property/fuzz test (export → read → replay = equivalent)
- [ ] Add concurrent `Close()` + `OnEvent()` race test
- [ ] Modernize benchmarks to `b.Loop()` (resolve ~46 gopls warnings)

## Tooling & CI

- [ ] Add `goreleaser check` job to CI workflow
- [ ] Add local `nix run .#check` all-checks command
- [ ] Add `govulncheck` to local devShell checks
- [ ] Set up dependabot for Go module dependency updates

---

## Deferred (needs consumer demand or architectural decision — see ROADMAP.md)

- [ ] OpenTelemetry span bridge (`attempt_start` → span start, `attempt_end` → span end)
- [ ] CLI tool (`auditlog`) for inspecting/replaying/diffing exported reports
- [ ] Time-based streaming flush (`WithFlushInterval(d time.Duration)`)
- [ ] Async channel-based streaming writer (backpressure decoupling)
- [ ] `MultiWriter` fan-out to multiple `OnEvent` callbacks
- [ ] `Diff()` on PeakConcurrency / CriticalPath (currently only duration delta)
- [ ] `FailureReason` structured categories (typed enum, not just a string)
