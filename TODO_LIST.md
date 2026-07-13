# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks. Derived from the 2026-06-21 status reports
after verifying against the codebase. Items already resolved by execution
sessions are marked `[DONE]`.

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

---

## [DONE] — 2026-06-23 execution session (v0.5.0 release)

- [x] **Adopt go-error-family for structured error classification** — all 14 sentinels
      classified (Corruption/Rejection/Transient/Infrastructure), 3 new I/O sentinels
      (`ErrReportLoadFailed`/`ErrRenderFailed`/`ErrExportWriteFailed`), 22 error paths wrapped,
      `ErrorClassifications()` + `RegisterClassifications(reg)` API, 18 error-path tests
- [x] **Tag v0.5.0 release** — tagged. CHANGELOG promoted, docs refreshed.
- [x] **Add `StepInfo.Type()` method** — API consistency with `Status.Label()`/`Icon()`/`Color()`
- [x] **Add retry/timeout columns to table export** — table now has 7 columns (Step, Status, Duration, Attempts, Retry, Timeout, Error)
- [x] **Add error-path tests for all Write\* methods** — 18 tests injecting failing `io.Writer`, unwritable dirs, bad input
- [x] **Document the table module `init()` registration pattern** — already in AGENTS.md:117
- [x] **Add `writeToFile` overwrite protection** — `CheckNoClobber(path)` + `ErrFileExists` sentinel
- [x] **Add benchmarks for render paths** — 6 benchmarks (WriteD2/Table/Tree/JSON/Mermaid 100-step + JSON 3-step)
- [x] **Add integration / round-trip tests** — JSON round-trip + NDJSON replay round-trip
- [x] **Add cross-format consistency tests** — Mermaid vs DOT vs D2 same nodes/edges
- [x] **Surface name collisions in diagrams** — `WorkflowReport.NameCollisions()` method
- [x] **Offer `Name(step)` fallback helper** — `HasPointerAddress(name)` detects unoverridden `String()`
- [x] **Add godoc `ExampleX` funcs** — `ExampleWorkflowReport_Duration` + `ExampleWorkflowReport_Filtered`
- [x] **Classification property tests** — wrapping preserves Family (200 iterations), identity matches map, fuzz target
- [x] **Strategy B migration path documented** in adoption report
- [x] **ROADMAP strategic designs** — all 6 strategic items scoped with first-chunk designs

---

## Short-term (high impact, low effort)

- [ ] **Add `WritePlantUMLString` coverage test** — currently at 80% coverage (unreachable error branch from `strings.Builder`). Low value — the 20% gap is a defensive error path that can't fire in practice.
- [ ] **Push coverage 93.9% → 95%+** — target `writeToFile` close/rename branches and `renderHTML` marshal error paths.
- [ ] **Add godoc `ExampleX` for `PeakConcurrency` and `CriticalPathDurationMs`** — 2 remaining examples (Duration and Filtered done).
- [ ] **Add godoc `ExampleX` for `WallClockDurationMs`** — field-level example (Duration uses `Duration()` method, field example is separate).

## Mid-term (medium impact, medium effort)

- [ ] **Make table columns configurable** — `WriteTable` currently hardcodes 7 columns. Add column-selection options. **Blocked**: needs upstream go-output `RenderOptions` support for column filtering. Design documented in ROADMAP.md.
- [ ] **Add diagram layout direction option** — let caller choose TD vs LR for Mermaid/D2/Graphviz. **Blocked**: needs upstream go-output renderer direction config. Design documented in ROADMAP.md.
- [ ] **Add fuzz tests for diagram ID sanitization** — arbitrary step names (special chars, unicode, empty) through go-output escape functions. `FuzzDiagramSpecialChars` exists but could go deeper into sanitization edge cases.

## Deferred (high effort or needs architectural decision — see ROADMAP.md)

- [ ] Split library into core (auditlog) + visualization (diagrams/tables) sub-modules
- [ ] Streaming NDJSON export option
- [ ] OpenTelemetry span bridge
