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

## [DONE] — 2026-07-22 execution session

- [x] **Add `WritePlantUMLString` coverage test** — happy-path test in `diagram_test.go` verifying non-empty output containing the step name.
- [x] **Push coverage 93.9% → 95%+** — reached **95.6%** by adding tests for `ExportTable` (0%→100%), `StepInfo.Type` (0%→100%), `StepStatus.IsKnown` (0%→100%), `RunID.String`/`IsEmpty` (0%→100%), `Summary` all 3 branches (83%→100%), `StepStatus.Color` unknown branch (67%→100%), `d2DiagramTitle` empty-ID fallback (67%→100%), `statusStyle` pending/canceled branches (75%→100%), `matchEvent` time-filter branches (75%→100%). Remaining gaps are unreachable defensive paths (strings.Builder/marshal errors on valid types).
- [x] **Add godoc `ExampleX` for `PeakConcurrency`, `CriticalPathDurationMs`, `WallClockDurationMs`** — 3 new examples in `benchmarks_test.go` (5 total now: Duration, Filtered, PeakConcurrency, CriticalPathDurationMs, WallClockDurationMs).
- [x] **Add fuzz tests for diagram ID sanitization** — new `FuzzDiagramSanitization_MultiStep` in `fuzz_test.go` with 17 seed pairs covering unicode (emoji, CJK, RTL, combining marks), control chars, diagram-keyword collisions, whitespace-only, length extremes, and edge (not just node) sanitization. 710K+ executions, zero crashes.

---

## [DONE] — 2026-07-22 execution session (mid-term features)

- [x] **Configurable table columns** — `WithColumns(TableColumn...)` option on `WriteTable`/`WriteTableString`/`ExportTable`. 10 columns available (Step, Status, Duration, Attempts, MaxAttempts, Retry, Timeout, Error, Type, Dependencies). Default preserves backward compatibility (original 7 columns). Uses pre-filter in `buildTableData` — no upstream go-output changes needed. New files: `table_options.go`, `table_columns_test.go`.
- [x] **Diagram layout direction** — `WithDirection(output.Direction)` option on all diagram writers (`WriteMermaid`/`WriteGraphviz`/`WriteD2`/`WritePlantUML` + String/Export variants). DOT and D2 use native go-output renderer support; Mermaid uses post-processing (`flowchart TD` → `flowchart LR/BT/RL`); PlantUML uses `left to right direction` injection. All methods accept backward-compatible variadic `opts ...DiagramOption`. New files: `diagram_options.go`, `diagram_direction_test.go`.

---

## Short-term (high impact, low effort)

_(all items completed — see 2026-07-22 session above)_

## Mid-term (medium impact, medium effort)

_(all items completed — see 2026-07-22 session above)_

## Deferred (high effort or needs architectural decision — see ROADMAP.md)

- [ ] Split library into core (auditlog) + visualization (diagrams/tables) sub-modules
- [ ] Streaming NDJSON export option
- [ ] OpenTelemetry span bridge
