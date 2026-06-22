# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks. Derived from the 2026-06-21 status reports
after verifying against the codebase. Items already resolved by the 2026-06-21
execution session are marked `[DONE]`.

Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).

---

## [DONE] — 2026-06-21 execution session

- [x] Populate CHANGELOG [Unreleased] (6 fields + ErrCountMismatch + go-output adoption)
- [x] Fix diagram edge direction → execution flow (dependency → step) to match tree
- [x] Add edge-direction regression tests (Mermaid + D2 + Graphviz + Tree)
- [x] Move `Duration()` and `Summary()` from diff.go to report.go (cohesion)
- [x] Make D2 diagram title derive from WorkflowID (not hardcoded)
- [x] Add `Write*String` methods to Auditor (mirror 7 WorkflowReport methods)
- [x] Add canonical JSON/NDJSON names on Auditor (`WriteJSON`/`ExportJSON`/etc.) + deprecated aliases
- [x] Add `Export*` methods to WorkflowReport (replays can write to file without Auditor)
- [x] Add PeakConcurrency high-fan-out stress test (8 parallel steps)
- [x] Add CriticalPath diamond DAG test (non-linear topology)
- [x] Document 3 duration metrics in README
- [x] Create FEATURES.md, TODO_LIST.md, ROADMAP.md
- [x] **Adopt go-error-family for structured error classification** — all 14 sentinels
      classified (Corruption/Rejection/Transient/Infrastructure), 3 new I/O sentinels
      (`ErrReportLoadFailed`/`ErrRenderFailed`/`ErrExportWriteFailed`), 22 error paths wrapped,
      `ErrorClassifications()` + `RegisterClassifications(reg)` API, 18 error-path tests

---

## Short-term (high impact, low effort)

- [ ] **Tag v0.5.0 release.** Error classification adoption (go-error-family v0.5.0),
      3 new I/O sentinels, 22 wrapped error paths, configurable table columns, diagram
      layout direction, and StepInfo.Type() API expansion warrant a minor bump.
      _(changelog populated, all docs refreshed, ready to tag)_
- [ ] **Add `StepInfo.Type()` method** — expose `StepType` via a method for consistency with `Status.Label()` / `Icon()` / `Color()`. Low effort, API consistency.
- [ ] **Add retry/timeout columns to table export** — `HasRetry` and `HasTimeout` are in `StepInfo` but not in the table. Low effort.
- [ ] **Add error-path tests for all Write\* methods** — inject failing `io.Writer`s, verify error wrapping. Most Write\* methods sit at ~83% coverage.
- [ ] **Add `WritePlantUMLString` coverage test** — currently 0% coverage per the 02:12 report.
- [ ] **Push coverage 93.2% → 95%+** — target `validateStatusCounts` branches and `omitempty` field paths.
- [ ] **Document the table module `init()` registration pattern** in AGENTS.md gotchas.

## Mid-term (medium impact, medium effort)

- [ ] **Make table columns configurable** — `WriteTable` currently hardcodes 5 columns. Add column-selection options.
- [ ] **Add diagram layout direction option** — let caller choose TD vs LR for Mermaid/D2/Graphviz.
- [ ] **Add `writeToFile` overwrite protection** — `os.Create` truncates unconditionally; add `O_EXCL` or "file exists" error option.
- [ ] **Add benchmarks for new render paths** — WriteD2, WriteTable, WriteTree, `finalizeDenormalized()` on 100+ step flows.
- [ ] **Add fuzz tests for diagram ID sanitization** — arbitrary step names (special chars, unicode, empty) through go-output escape functions.
- [ ] **Add integration / round-trip tests** — report → JSON → LoadReport → report → diagram → structural verify.
- [ ] **Add cross-format consistency tests** — same report → Mermaid vs DOT vs D2 should have same node/edge structure.
- [ ] **Surface name collisions in diagrams** — when two steps share `String()`, the `seen` map silently merges them; should warn.
- [ ] **Offer `Name(step)` fallback helper** — falls back to type name when `String()` returns a pointer address.
- [ ] **Add godoc `ExampleX` funcs** for `WallClockDurationMs`, `PeakConcurrency`, `CriticalPathDurationMs`.

## Deferred (high effort or needs architectural decision — see ROADMAP.md)

- [ ] Migrate `justfile` → `flake.nix` (build automation)
- [ ] Split library into core (auditlog) + visualization (diagrams/tables) sub-modules
- [ ] Streaming NDJSON export option
- [ ] OpenTelemetry span bridge
- [ ] `encoding/json/v2` migration
