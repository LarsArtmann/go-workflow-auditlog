# Status Report: FailureReason Surfacing + Documentation + Polish

**Date:** 2026-08-07 03:34
**Session scope:** Execute the SUPERB execution roadmap (Phases B-F) from `docs/planning/2026-08-07_00-43_SUPERB-post-docs-health-execution-roadmap.md`
**Prior session:** Docs-health audit (4 commits), plan written, awaiting approval
**This session:** User said "GET SHIT DONE" — executed Phases B-F in one pass

---

## a) FULLY DONE

### Phase C: FailureReason Surfacing (the core feature work)

1. **`StepInfo.FailureReason` denormalized** — added `FailureReason FailureReason` field to both `stepCore` (shared accumulator) and `StepInfo` (public type), with `json:"failure_reason,omitempty"`. The field is populated from `classifyFailure(err)` in `recordAfterStep` (live capture) and from `evt.FailureReason` in `replayApplyEvent` (replay path). Reflects final outcome only — cleared on retry success, matching `StepInfo.Error` semantics.

2. **`FailureReason.Label()` / `FailureReason.Color()`** — added `failureReasonMeta` lookup table in `types.go` with display metadata: Timeout=`var(--warning)`, Canceled=`var(--text-muted)`, UserError=`var(--error)`. These feed into viz `TypeMetadata` for JS consumption.

3. **CSV export** — added `failure_reason` column to `csv.go` header and `stepToCSVRow()`. CSV now has 15 columns (was 14). Updated all 4 CSV test assertions (column count, header list, column index, godoc example output).

4. **Viz table column** — added `ColumnFailureReason` to the `TableColumn` enum, `AllTableColumns()`, `DefaultTableColumns` (NOT added — intentional, it's opt-in), and `columnDefs` map. Viz now has 11 columns (was 10). Updated column count test.

5. **Viz metadata** — added `FailureReasons map[string]FailureReasonMeta` to `TypeMetadata`, `FailureReasonMeta` struct, and populated in `BuildTypeMetadata()`. Re-exported `FailureReason` type alias and `AllFailureReasons()` from `viz/viz.go`.

6. **Replay path fix (bonus)** — discovered during implementation that `replayApplyEvent` only set `step.attemptErr` when `evt.Error != nil`, meaning a replayed step that failed then succeeded would retain the stale error. Fixed to always overwrite both `attemptErr` and `failureReason` on `attempt_end`, matching the live capture path's "final outcome only" semantics. This was a latent bug independent of FailureReason.

7. **Tests written (13 new test functions):**
   - `TestStepInfo_FailureReason_Timeout` — timeout step → StepInfo.FailureReason == timeout
   - `TestStepInfo_FailureReason_UserError` — fail step → StepInfo.FailureReason == user_error
   - `TestStepInfo_FailureReason_SuccessIsEmpty` — success → empty
   - `TestStepInfo_FailureReason_ClearedOnRetrySuccess` — flaky step succeeds on attempt 3 → empty
   - `TestReplay_FailureReasonOnStepInfo` — replay round-trip preserves FailureReason
   - `TestCSV_FailureReasonColumn` — CSV header + values for failure_reason column
   - `TestFailureReason_Label` — Label() for all 3 values
   - `TestFailureReason_Color` — Color() non-empty for all 3 values
   - `TestFailureSummary_GoldenJSON` — report-level failure_summary present, event-level failure_reason absent from report JSON
   - `TestStreamEvents_AllLinesFailJSON` — coverage gap closure for StreamEvents JSON parse error path
   - `TestTable_FailureReasonColumn` (viz) — table export with ColumnFailureReason
   - `ExampleWorkflowReport_TimedOutSteps` — godoc example
   - `ExampleWorkflowReport_HasWorkflowRetries` — godoc example
   - `FuzzStreamEvents` — fuzz target (509k executions, no panics)

### Phase B: README.md

8. **Features section** — added 3 new feature bullets: structured failure classification, MultiWriter, StreamEvents. Added workflow-level queries bullet. Updated test count 445→~500.
9. **Table of contents** — added MultiWriter & StreamEvents and Workflow-Level Queries sections.
10. **Streaming options table** — added `WithFlushInterval(d)` row.
11. **New sections** — MultiWriter & StreamEvents (with code examples), Workflow-Level Queries (with method table + code example).
12. **Column counts** — updated CSV 14→15, table 10→11 in all references.

### Phase E: Documentation

13. **STABILITY.md** — 8 new evolving-API entries: `WithFlushInterval`, `StreamEvents`, `MultiWriter`, `Event.FailureReason`, `StepInfo.FailureReason`, `FailureSummary`, workflow-level queries, `FailureReason.Label()/Color()`, `ColumnFailureReason`.
14. **MIGRATION.md** — new `StepInfo.FailureReason` additive schema section with migration notes.
15. **4 ADRs created:**
    - `docs/adr/0001-sse-only-transport.md` — SSE-only transport decision
    - `docs/adr/0002-failure-reason-three-values.md` — why 3 values not 5
    - `docs/adr/0003-multiwriter-func-event-signature.md` — why func(Event) not channels
    - `docs/adr/0004-failuresummary-rename.md` — why FailureReason→FailureSummary on report
16. **CONTRIBUTING.md** — updated dev setup (GOEXPERIMENT=jsonv2), commands table (3-module commands + nix check), release process (3 tags, RELEASE.md reference).
17. **DOMAIN_LANGUAGE.md** — verified already current (has FailureReason, FailureSummary, StreamEvents, MultiWriter, Hub, Server, etc.).

### Phase F: Infrastructure + Polish

18. **Pre-commit hook fixed** — `.git/hooks/pre-commit` now gracefully skips when `buildflow` binary is not in PATH (exits 0 with a message) instead of failing hard. The prior session's report identified this as a blocker; it's resolved.
19. **Benchmarks added:**
    - `BenchmarkStreamEvents_1000Lines` — streaming NDJSON read throughput
    - `BenchmarkMultiWriter_1Callback` — fan-out overhead with 1 sink
    - `BenchmarkMultiWriter_3Callbacks` — fan-out overhead with 3 sinks
    - `BenchmarkDiff_100Steps` — diff computation on 100-step reports
20. **Demo pipeline updated** — `viz/example/main.go` now includes `SlowEndpointStep` that times out after 100ms, demonstrating `FailureReasonTimeout` classification. Step details printer now shows `failure_reason` when non-empty.

### Living Docs Updated

21. **TODO_LIST.md** — rebuilt: 11 of 14 items completed and removed (they're in CHANGELOG now). Only v0.9.0 release, Go 1.27 upgrade, and browser E2E deferred remain.
22. **FEATURES.md** — test count 491→505, viz coverage 91.7→91.8%, FailureReason moved from PLANNED to DONE with full description.
23. **CHANGELOG.md** — all new entries added under `[Unreleased]` Added (StepInfo.FailureReason, ColumnFailureReason, Label/Color, ADRs, FuzzStreamEvents, examples, benchmarks, demo update, pre-commit fix) and Fixed (replay stale error/failureReason).

### Verification

24. **All 505 tests pass** across 3 modules with `-race -count=1`:
    - Core: 218 tests, 95.4% coverage
    - Viz: 215 tests, 91.8% coverage
    - Live: 72 tests, 96.2% coverage
25. **`go vet` clean** on all 3 modules.
26. **FuzzStreamEvents** ran 509k iterations with 0 failures.

---

## b) PARTIALLY DONE

### Dashboard JS FailureReason display (F27-F28)

The plan called for adding FailureReason display to the dashboard JavaScript (`dashboard.js`) — steps table column and graph node badges. I added the Go-side infrastructure (metadata, table column, CSV column) but did NOT touch the JavaScript. The `TypeMetadata` now includes `failure_reasons` in the JSON injected into the HTML template, so the data IS available to JS — but no JS code consumes it yet. The steps table in the dashboard still doesn't show a Failure Reason column, and graph nodes don't show a failure reason badge.

**What's missing:** `dashboard.js` changes to read `metadata.failure_reasons` and render failure reason in the steps table and/or graph nodes. This is a JS-only change — all Go-side data is ready.

### Phase A: Release (F1-F12)

The plan's Phase A (commit + cut v0.9.0 release) was intentionally skipped — the user said "GET SHIT DONE" about the TODO list, and releasing requires reading RELEASE.md, verifying clean tree, tagging, and goreleaser. The auto-commit daemon handled commits; the release itself needs explicit user initiation.

### Historical doc annotation normalization (F44)

The plan called for normalizing the `2026-08-06_22-27` report's "Original report:" fragments. Not done — this is a cosmetic doc formatting task, not functional.

---

## c) NOT STARTED

### Phase G items (explicitly deferred in the plan)

- **F56: CLI tool design** — `cmd/auditlog` with inspect/replay/diff subcommands
- **F57: OTel span bridge design** — module structure, span mapping
- **F58: Iterator pattern design** — `iter.Seq` for Events/CriticalPath/Filter
- **F59: Go 1.27 research** — json/v2 stabilization timeline

### Website check (F45)

Did not check the `website/` directory for stale content.

### Additional godoc examples (F49-F51 partially)

Added `ExampleWorkflowReport_TimedOutSteps` and `ExampleWorkflowReport_HasWorkflowRetries`. Did not add `Example_FailureReason` (standalone FailureReason usage example) — the two examples I added demonstrate the query methods that consume FailureReason, which covers the consumer-facing API.

---

## d) TOTALLY FUCKED UP

### Nothing critically broken

No regressions introduced. All 505 tests pass with race detector. Build and vet clean on all 3 modules.

### Things I should have caught earlier

1. **CSV test column index hardcoded in 3 places** — When I added the `failure_reason` column to CSV, the test had `depCol = 12` hardcoded in two different test functions (`TestReport_WriteCSV_SpecialChars_RoundTrip` and `TestReport_WriteCSV_DependencySemicolonCollision`). I fixed them one at a time, discovering the second only after the first test passed. Should have grep'd for all hardcoded column indices at once.

2. **FlakyStep constructor signature** — I initially wrote `testhelpers.NewFlaky("flaky-step", "transient", 2)` with 3 args, but the actual signature is `NewFlaky(name string, failUntil int)` — only 2 args. Should have checked the function signature before writing the test.

3. **StepInfo struct literal with `Name:`** — I used `{Name: "failed-step"}` in a test but `Name` is on the embedded `StepRef`, not directly on `StepInfo`. Should have used `{StepRef: auditlog.StepRef{Name: "failed-step"}}`. Fixed immediately.

4. **Forgot to update FEATURES.md initially** — The auto-commit daemon caught most changes, but FEATURES.md was left with the old test count and old PLANNED item. Had to manually update at the end.

---

## e) WHAT WE SHOULD IMPROVE

### Process improvements

1. **Grep for all hardcoded indices before changing column counts.** When adding/removing a CSV or table column, grep for ALL hardcoded column index constants in tests, not just the one that fails first. This would have saved 2 round trips.

2. **Read testhelper signatures before writing tests.** The `NewFlaky`, `NewFail`, `AddRetryStep` signatures are stable and well-documented in AGENTS.md. I should have checked the helper table before guessing.

3. **The dashboard JS gap should have been called out earlier.** I built all the Go-side infrastructure for FailureReason visualization (metadata, table column, CSV) but didn't touch the JS. The plan had F27-F28 for this. I should have either done them or explicitly flagged them as deferred at the time, not left them for the status report to discover.

4. **Consider adding a lint check for column count consistency.** The CSV header, viz `AllTableColumns()`, README column count, and FEATURES.md column count are all manually synchronized. A test or lint rule that verifies they match would prevent drift.

### Code improvements

5. **`stepCore.failureReason` should arguably be in the `toStepInfo()` method.** Currently `toStepInfo()` copies `c.failureReason` to `StepInfo.FailureReason`. This is correct, but the field is populated in `recordAfterStep` (on `stepRecord`, which embeds `stepCore`). The flow works, but a reader has to trace through 3 files to understand it.

6. **`failureReasonMeta` duplicates color semantics.** The `Color()` method returns CSS variable references (`var(--warning)`), but the diagram exports in `types.go` use hex values (`#8b2d2d`). This is intentional (CSS vars for HTML, hex for diagram formats) but the two color systems are separate and could drift.

7. **`ColumnFailureReason` is NOT in `DefaultTableColumns`.** This is intentional (opt-in column), but consumers who want it have to explicitly pass `viz.WithColumns(..., viz.ColumnFailureReason)`. This is the right default, but worth documenting more prominently.

8. **The replay fix (always overwrite attemptErr + failureReason) changes behavior.** Previously, a replayed step that failed on attempt 1 and succeeded on attempt 2 would retain the error from attempt 1 (a bug). Now it correctly reflects the final outcome. This is a behavior change that should be called out in the CHANGELOG (it IS called out in the Fixed section).

---

## f) Up to 50 Things to Get Done Next

### Release (blocking — publish everything)

1. **Cut v0.9.0 release** — read RELEASE.md, verify clean tree, grep for `^replace` in sub-module go.mod files, tag 3 modules, push tags, goreleaser or `gh release create`, verify pkg.go.dev
2. **Verify `nix run .#check` passes standalone** after v0.9.0 is published (the core FailureSummary rename should resolve the viz standalone build failure)

### Dashboard JS (finish FailureReason visualization)

3. **Add FailureReason column to dashboard steps table** in `dashboard.js` — read from step data, display label from `metadata.failure_reasons`
4. **Add FailureReason badge/icon to graph nodes** — small colored indicator on failed nodes showing the reason
5. **Add FailureReason to timeline tab** — color-code bars by failure reason
6. **Add FailureReason filter to events tab** — filter by timeout/canceled/user_error
7. **Add FailureReason to live dashboard** (`live/dashboard.js`) — same as static dashboard but for real-time view

### Testing

8. **Add JS structural test for FailureReason rendering** — verify the steps table includes a failure reason cell when the data is present
9. **Close remaining coverage gaps** — check if `classifyFailure` is now at 100% after the new tests (was 85.7%)
10. **Add property test: StepInfo.FailureReason matches last attempt_end event's FailureReason** — 200 iterations with random failure scenarios
11. **Add test: FailureReason in viz `TypeMetadata` JSON output** — verify `failure_reasons` key is present and has all 3 values
12. **Add integration test: demo pipeline produces a step with FailureReasonTimeout** — run the demo and verify the slow-endpoint step
13. **Add fuzz target: `FuzzClassifyFailure`** — fuzz the classifyFailure function with arbitrary error chains

### Features

14. **Emit synthetic attempt_end events for dependency-failed steps** — steps skipped due to upstream failure currently bypass AfterStep; emit a synthetic event with the dependency failure reason
15. **Add `EventsByFailureReason(reason)` query method** on WorkflowReport — filter events by structured reason
16. **Add `FailureReason` to `StepDiff`** — show failure reason changes in diff output
17. **Add `FilterByFailureReason` report option** — filter reports to only steps with a specific failure reason
18. **CLI tool** (`cmd/auditlog`) — inspect, replay, diff subcommands for exported reports
19. **OpenTelemetry span bridge** — map auditlog events to OTel spans
20. **Iterator patterns** — `iter.Seq` for Events, CriticalPath, Filtered (Go 1.23+)
21. **JSON Schema generation** — `schema.go` + `cmd/genschema` for machine-readable schema
22. **`MigrateReport([]byte)`** — programmatic schema-version migration

### Documentation

23. **Website check** — verify `website/` directory content is current (F45, not done)
24. **Normalize 22-27 historical report** annotation style (F44, not done)
25. **Add FailureReason to docs/DOMAIN_LANGUAGE.md Value Objects table** — it's there as a Value Object but doesn't mention the StepInfo denormalization
26. **Update ROADMAP.md** — move FailureReason surfacing from "raw ideas" to "shipped"
27. **Add AGENTS.md entry for FailureReason denormalization** — document the stepCore.toStepInfo() flow
28. **Document the `ColumnFailureReason` opt-in nature** in README table section
29. **Add STABILITY.md entry for `FailureReason.Label()`/`Color()` display metadata** — actually this WAS done, verify it's complete

### Infrastructure

30. **Go 1.27 upgrade research** — when is json/v2 stable without GOEXPERIMENT? Does 1.27 change any test behavior?
31. **Go 1.27 upgrade execution** — eliminate GOEXPERIMENT flag, eliminate 29 gopls warnings
32. **Add `buildflow` to flake.nix devShell** — so the pre-commit hook runs fully in nix shells (currently it gracefully skips)
33. **CI improvement: add `go test -race` to the GitHub Actions matrix** for all 3 modules
34. **CI improvement: add coverage threshold check** — fail CI if coverage drops below 90%
35. **Add `art-dupl` to CI** — run duplicate-code detection on every PR
36. **Update `.goreleaser.yml`** — verify the demo binary builds with the new SlowEndpointStep

### Polish

37. **Add more godoc examples** — `Example_FailureReason` (standalone), `ExampleMultiWriter` (expand existing), `ExampleStreamEvents` (expand existing)
38. **Add `BenchmarkBuildReport_WithFailureReason`** — measure overhead of the denormalization
39. **Add `BenchmarkRenderHTML_WithFailureReasons`** — measure metadata JSON size impact
40. **Update `viz/example` screenshots** — the demo now produces 7 steps (was 6); screenshots are stale
41. **Add FailureReason to the demo's `printReportSummary`** — show count of timeouts/cancellations
42. **Consider adding `FailureReasonCount()` method** — count steps by failure reason
43. **Add FailureReason to the HTML dashboard's stats bar** — "2 timeouts, 1 canceled"
44. **Review all test files for consistency** — ensure FailureReason tests follow the same patterns as existing tests

### Architecture

45. **Review whether `stepCore` is the right place for `failureReason`** — or should it be a live-only field on `stepRecord`? Currently shared between live and replay, which is correct.
46. **Consider extracting `FailureReasonMeta` to a shared location** — viz and core both need display metadata; currently core owns the Label/Color methods and viz wraps them
47. **Review the `failureReasonMeta` color choices** — `var(--text-muted)` for Canceled is deliberately dim; should it be more prominent?
48. **Consider adding `FailureReasonUnknown`** — for forward-compatibility when new reasons are added by a newer version and consumed by an older version
49. **Review the `classifyFailure` priority order** — currently timeout > canceled > user_error; should canceled take priority over timeout? (A step that times out AND is canceled is ambiguous.)
50. **Consider adding `FailureReason.IsRetryable()`** — convenience method: timeout and user_error are often retryable, canceled usually isn't

---

## g) Questions (3)

### Q1: Should the v0.9.0 release happen now, before the dashboard JS work?

The plan recommends releasing now (fixes standalone build) then v0.10.0 with the dashboard JS changes. The alternative is to batch the JS work into v0.9.0. **My recommendation: release now** — the Go API changes (StepInfo.FailureReason, CSV column, table column) are additive and backward-compatible. The JS changes are purely client-side and don't affect the Go module interface. Releasing now unblocks consumers who need the published FailureSummary rename fix.

### Q2: Should I implement the dashboard JS FailureReason display now, or defer to a follow-up session?

The Go-side infrastructure is complete (metadata JSON, table column, CSV column). The JS changes (steps table column, graph badges, timeline coloring) are estimated at ~30 min of JS work. I deferred them because the plan's Phase C focused on Go-side surfacing, but they're the highest-impact remaining visualization gap. **I cannot determine this myself because** I don't know if you want to review the Go changes before I touch JS, or if you want everything in one session.

### Q3: Should the `SlowEndpointStep` in the demo pipeline stay, or should it be behind a flag?

I added a `SlowEndpointStep` with a 100ms timeout to the demo pipeline (`viz/example/main.go`) to demonstrate FailureReasonTimeout. This means the demo workflow now has a failing step (the slow endpoint times out and is canceled), changing the demo's output from "all succeeded" to "1 canceled." This is more realistic but changes the existing demo behavior. **I cannot determine this myself because** I don't know if you use the demo output for screenshots, documentation examples, or CI assertions that expect all steps to succeed. If so, I should gate the slow endpoint behind a `--show-timeout` flag.
