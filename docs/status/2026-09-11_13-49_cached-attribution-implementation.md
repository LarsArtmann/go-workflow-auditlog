# Status Report: Cached Concept — Honest Cache-Hit Attribution

**Date:** 2026-09-11 13:49
**Session start state:** clean tree on `master` @ `e042b45`
**Session end state:** clean tree @ `7745fc6` — the auto-commit daemon committed all session work (3+ daemon commits)
**Verdict:** Core feature COMPLETE and green (test + race + vet + lint across all 3 modules). Docs partially updated. BuildFlow consumer wiring NOT started (blocked on next release by design).

---

## Origin

User asked whether "Cached" exists, pointing at `/home/lars/projects/BuildFlow` and an exported BuildFlow audit log (`workflow-audit-log-20260911-080328-e614c0a7.html`). Findings:

- `StepStatus` enum = pending/running/succeeded/failed/canceled/skipped — no cached. The exported HTML had 209× succeeded / 21× skipped / 4× failed, zero "cached" strings.
- BuildFlow HAS a result cache (`execution/result_cache.go`, `cache/` pkg). On a cache hit the detect step still runs `cachedDetect` and is recorded as plain **succeeded** — indistinguishable from fresh execution. Only trace: a TUI progress message `"result cache hit"` (pipeline_builder.go:185) + aggregate `CacheStats` on WorkflowResult. Nothing per-step in the audit trail.
- Structural constraint: `flow.StepStatus` has no cached concept, and it can never arrive via Before/AfterStep callbacks — caching is a consumer-level concern.

User's directive: "Maybe we need to add this!??!" + "I hate lying to users … not giving them the FULL information." → Design principle: **cached is NOT a status** (a cache hit IS a success; smashing the axes breaks status filtering). It is orthogonal attribution: WHERE the result came from.

---

## a) FULLY DONE ✅

### Design (data-model-first)

- **`Event.Cached bool`** (JSON `cached,omitempty`) — attempt_end carries it; attempt_start never does (tested).
- **`StepInfo.Cached bool`** — any-attempt semantics; per-attempt truth stays in the event stream (same precedent as `Error`/`FailureReason`).
- **`stepCore.cached`** — shared accumulator so live capture and replay can never diverge (project rule: new step-state fields go on stepCore).
- **`WorkflowReport.CachedStepCount`** (JSON `cached_step_count,omitempty`) — denormalized in `finalizeDenormalized`, recomputed by Filtered/MigrateReport, validated.
- **Public API `auditlog.MarkCached(ctx) error`** — context-based (exact step identity, no name-collision risk; BuildFlow already threads ctx into `detectWithCache`). BeforeStep now wraps ctx via `withStepContext` (a `context.WithValue` wrapper that preserves deadlines/cancellation — step timeouts keep working).
- **`ErrMarkCachedNoStepContext`** — Rejection sentinel (intrinsic family, code `auditlog.mark_cached_no_step_context`, registered in `ErrorClassifications`). Returned when ctx wasn't injected (auditing disabled / outside step body) — documented as non-fatal for consumers.

### Core implementation (all files)

| File | Change |
|---|---|
| `cached.go` (NEW) | `MarkCached`, `ErrMarkCachedNoStepContext`, `stepContext`/`stepContextKey`, `withStepContext`, `Recorder.markCached` (mutex-guarded) |
| `event.go` | `Cached` field + `WasCached()` predicate |
| `step.go` | `stepCore.cached`, `StepInfo.Cached` + doc, `toStepInfo` propagation |
| `attach.go` | BeforeStep injects step identity into ctx |
| `recorder.go` | attempt_end event carries `Cached: rec.cached` (retry-correct: attempt 1 false, attempt 2 true) |
| `replay.go` | `replayApplyEvent` sets `step.cached` from attempt_end → NDJSON round-trip preserves flag |
| `report.go` | `CachedStepCount` field + `validateCachedStepCount` (`ErrCountMismatch` on drift) |
| `report_builder.go` | `finalizeDenormalized` computes the count |
| `filter.go` | `WithCachedSteps()` / `WithUncachedSteps()` ("what did we NOT re-verify?" / "what ran fresh?"); cached filter also restricts events to kept steps |
| `csv.go` | `cached` column (after has_timeout) in CSV + TSV |
| `classify.go` | sentinel registered (Rejection) |
| `cmd/auditlog/info.go` | `cached: N (results reused, not re-verified)` breakdown + `⚡cached` marker per step in step list |
| `testhelpers/testhelpers.go` | `CachedStep` type + `NewCached(name)` (succeeds by "reusing a stored result", calls `MarkCached`) |
| `schema/report.schema.json` | regenerated via `cmd/genschema` (+9 lines) |

### Viz module

- `ColumnCached` table column (appended — existing iota values stable), `AllTableColumns()` → 12.
- `dashboard.js`: `⚡ cached` config-badge (title: "Result served from a cache — the step did not re-execute its work"), "cached" search token on step rows, `⚡ cached` chip on events rows, graph-node `⚡` badge (top-left, avoids retry-badge collision, aria-label).
- `dashboard.css`: new `--cache`/`--cache-dim` color tokens (violet, distinct from all existing), `.config-badge.cached`, `.cached-chip`.

### Live module

- `dashboard.js`: SSE `attempt_end` ingestion sets `step.cached`; new-step state init includes `cached: false`; shared `configBadgesHTML()` helper (create + update paths render identically); `stepStateKey` includes cached (row updates when flag arrives mid-run); `updateStepRow` rebuilds config cell; events chip; graph badge; node aria-label gains "(result from cache)".
- Live reuses viz base CSS (embedded first), so shared badge styles apply automatically.

### Tests — all green with `-race`; lint 0 issues × 3 modules; vet clean

- **NEW `cached_test.go` (12 tests):** marks step+event+count; foreign ctx → sentinel; disabled auditor → sentinel surfaces (step fails visibly, not silently); no cross-step leak (parallel steps); 16 concurrent steps (race-safe); retry-then-cached per-attempt truth (attempt 1 executed fresh=false, attempt 2 cached=true, step error cleared); defaults-false + `"cached":true` omitted from JSON; JSON round-trip (load+validate); NDJSON replay round-trip; both filter directions + event-stream restriction; validate detects count drift; MigrateReport recomputes stale count; `WasCached` predicate.
- **NEW godoc `ExampleMarkCached`** (uses `WithUncachedSteps`, prints "N of M steps served from cache").
- **NEW viz tests:** `TestTable_CachedColumn`, `TestWriteHTML_CachedStepHonesty` (pins data flow: `"cached":true` in embedded report JSON + all three badge mechanisms in embedded JS).
- **NEW live tests:** `TestDashboardJS_CachedAttribution` (7 required patterns), `TestServer_SSE_CachedEventFlag` (SSE payload carries `"cached":true`).
- **UPDATED for the new CSV column:** header list, TSV 15→16 cols, depCol 13→14 (×2), failure_reason col 12→13, `ExampleWorkflowReport_WriteCSV` output (3rd `false`), `AllTableColumnsCount` 11→12.

### Docs

- `CHANGELOG.md`: full `[Unreleased]` → Added entry.
- `FEATURES.md`: `Cached` bullet in Report Aggregate Fields (DONE section).
- `example_test.go` `finalizeForExample`: now computes `CachedStepCount`.

---

## b) PARTIALLY DONE ⚠️

### AGENTS.md update — LOCATED but NOT APPLIED (interrupted mid-flight)

I had just identified the exact edit points when the session was cut for this report:

1. Core file map: missing `cached.go` entry.
2. Gotcha section: missing the Cached-concept entry (ctx injection mechanics, non-fatal sentinel contract, any-attempt semantics, why it is NOT a StepStatus).
3. Table export bullet: says "10 columns available" — now 11 (+`ColumnCached`).
4. Test metrics: says "481 test functions (core: 212; viz: 197; live: 72)" — **stale before this session** (my counts incl. Example/Benchmark/Fuzz: core 250, viz 233, live 74 = 557 total; +17 from this session). The old methodology is unknown (likely `func Test` only) — recount needed with a documented method.

### Verification

Individual gates all ran green (`go test -race`, `go vet`, `golangci-lint` per module). The consolidated `nix run .#check` gate (adds govulncheck) was NOT run this session. No new dependencies were added, so vuln risk is nil — but the gate should still run before release.

---

## c) NOT STARTED ⛔

1. **BuildFlow wiring** (the actual consumer): `detectWithCache` cache-hit branch (pipeline_builder.go:183-191) should call `auditlog.MarkCached(ctx)`. BLOCKED by design: BuildFlow pins published auditlog v0.9.0; consuming the new API needs the next auditlog release first (or a temporary go.work `use`). BuildFlow-side extras: nolint:erraudit comment for the disabled-audit path, e2e test (run 2 → assert `"cached":true` in audit JSON), TUI/summary surfacing, OTel span attribute.
2. **`docs/MIGRATION.md`** — no section for the additive fields (`Event.cached`, `StepInfo.cached`, `cached_step_count`). Additive-only (no rename/break), but the CHANGELOG entry references schema additions and MIGRATION.md is the canonical place.
3. **README.md** — no feature mention / usage snippet for `MarkCached`.
4. **Release** (v0.11.0 per RELEASE.md: CHANGELOG cut, three annotated tags, GORELEASER_CURRENT_TAG, clean-tree check, gh release, pkg.go.dev probe). The repo has a `release` skill for exactly this.
5. **Dashboard hero stats** — report/CLI show the cached count, but neither dashboard's summary stat row surfaces `cached_step_count` (only per-step badges + search).
6. **Diff() cached deltas** — `DiffResult` has CriticalPath/PeakConcurrency deltas but no cached delta; "cached went from 3 → 40 steps between runs" is precisely the regression class Diff exists to catch.
7. **Diagram exports** — Mermaid/DOT/D2/PlantUML/tree show nothing cached-specific (deliberate: cached is not a status → no color change; a `⚡` label suffix would be the honest middle ground if wanted).
8. **Live demo / viz example** — no cached step in the demo pipelines to showcase the badge.
9. **erraudit** — not run this session (private repo; all new error paths follow the dual-wrap/idiom policy by construction; one new sentinel).
10. **art-dupl** — threshold sweep not run after adding `CachedStep` (near-clone of SucceedStep by design; likely needs `//art-dupl:accept` or is below threshold).
11. **Coverage %** — not re-measured after cached.go (new code is heavily tested; formal % unknown).

---

## d) TOTALLY FUCKED UP 💥 (all caught and fixed in-session; listed for honesty)

1. **`report.go` multiedit MANGLED `validateCachedStepCount`** — two overlapping edits (one failed, one applied) left a duplicated loop body and a truncated function. Caught by reading back the file immediately; repaired; verified by tests+lint. Lesson: never fire two edits whose regions overlap; read back after multi-edits on one function.
2. **First draft of `cached_test.go` was wrong twice over:** concurrent `w.Add` from goroutines (Add is not thread-safe) and `flow.Step(step, RetryOpts(2))` (RetryOpts is a RetryOption mutator → must be `flow.Step(step).Retry(...)`). Both fixed before first run — but I wrote them wrong because I guessed API shapes instead of reading the helpers first.
3. **`ExampleMarkCached` initially wrong** — `finalizeForExample` didn't compute `CachedStepCount`, so the example printed "0 of 2". Fixed by extending the helper (it exists precisely to emulate finalize for examples; my new field belonged there from the start).
4. **`step.go` gofmt breakage** — the multi-line doc comment inside the struct split field-alignment groups; `gci` flagged it. Fixed with `gofmt -w`.
5. **My `//nolint:exhaustruct` was unnecessary** in cached_test.go (nolintlint: unused directive) — removed.
6. **Pre-existing noise I did not fix (correctly out of scope):** gopls `stdversion` warnings (jsontext vs go1.26 with GOEXPERIMENT=jsonv2), stale LSP "unused: withStepContext" cache, one pre-existing unused var in failure_reason_test.go.

---

## e) WHAT WE SHOULD IMPROVE (incl. "what did you forget / could have done better")

1. **I forgot AGENTS.md until the end** — the global memory protocol says update at the moment of discovery, not end-of-session. Deferred → interrupted → partially done. Should have written the file-map entry the minute `cached.go` was created.
2. **AGENTS.md contains a CONTRADICTION with reality:** the Gotcha claims `nlreturn` was removed and must not be re-enabled — yet this session's lint run flagged and I fixed 2 `nlreturn` violations, i.e. **nlreturn IS active in `.golangci.yml`**. Either the config was re-enabled later without updating AGENTS, or the removal never happened. One of the two documents is lying; verify and fix.
3. **AGENTS test metrics were already stale before me** (claimed 481; pre-session reality was ~540 by my counting method). Metrics in prose rot — consider a checked-in script that prints the counts (or a doc-test) instead of hand-maintained numbers.
4. **Diff() gap (design):** I consciously deferred cached deltas, but Diff's whole purpose is run-to-run honesty; a cache-rate regression is invisible to it. Should be next-priority feature work, not a someday item.
5. **Release cadence friction:** BuildFlow (the only real consumer, and the origin of this feature) cannot use the feature until a release is cut. The feature is invisible to its intended beneficiary while it sits unreleased. Consider cutting v0.11.0 promptly.
6. **I guessed APIs before reading helpers** (see d.2) — the workflow doc literally says read before write; I did it for production code but skipped it for test wiring. Cost: one rewrite. Cheap lesson, still a lesson.
7. **Editing hygiene:** one mangled multi-edit (d.1). For surgical single-region changes, single `edit` calls with read-back beat clever multiedits.
8. **Surfaces consistency:** CLI `info` prints the honest "results reused, not re-verified" phrasing; dashboards show badges but no aggregate. Full-information principle argues for the hero stat everywhere the count exists.
9. **No cached toggle in dashboards** — search token "cached" works, but an explicit "Cached only" chip (like "Errors only") would match the established interaction pattern.

---

## f) NEXT — up to 50 tasks, roughly Pareto-ordered

**Close out this feature (P0):**
1. Finish AGENTS.md: file map + Cached Gotcha + column count 10→11 + accurate test counts (decide counting method and state it).
2. Resolve the AGENTS-vs-.golangci.yml `nlreturn` contradiction (verify config; fix the lying doc).
3. docs/MIGRATION.md: additive `cached` fields section (+ schema regen note).
4. README.md: feature bullet + `MarkCached` snippet in the usage section.
5. Run `nix run .#check` (full gate incl. govulncheck) — pre-release anyway.
6. Run `golangci-lint config verify` only if .golangci.yml changes (it didn't this session).
7. Run erraudit locally on the three modules (if installed) to keep the zero-violation streak honest for the new error path.
8. Run art-dupl sweep; annotate/extract around `CachedStep` if flagged.
9. Re-measure coverage; target ~100% on `cached.go` (it's small and fully exercised — should already be close).

**Ship it (P0/P1):**
10. Cut release v0.11.0 via the repo's `release` skill / RELEASE.md (three tags, GORELEASER_CURRENT_TAG, clean tree, gh release, pkg.go.dev probe).
11. Bump BuildFlow deps to the new auditlog version.

**BuildFlow wiring (P1 — the reason this exists):**
12. `detectWithCache`: call `auditlog.MarkCached(ctx)` in the cache-hit branch (ctx is already a parameter).
13. Add the erraudit-compliant discard (`_ = auditlog.MarkCached(ctx) //nolint:erraudit // no-op when audit logging is disabled`).
14. BuildFlow e2e test: two consecutive runs → second run's audit JSON asserts `"cached":true` on detect steps.
15. Surface cached count in BuildFlow's run summary next to CacheStats hit-rate.
16. OTel sink: emit `auditlog.step.cached` span attribute on cached attempt_end.
17. Regenerate a real BuildFlow audit-log HTML and eyeball the badges (the file that started this session).

**Feature depth (P1/P2):**
18. Diff(): `CachedStepsAdded`/`CachedStepsRemoved` + `CachedCountDelta` in `DiffResult`/`StepDiff` (cached-state changes between runs).
19. Property tests for the new Diff duality (mirrors existing 8 Diff algebra properties).
20. Dashboard hero stat: "N from cache" card in viz + live summary rows (uses `cached_step_count`).
21. "Cached only" filter chip in both dashboards (pattern: errors-only chip).
22. Diagram exports: optional `⚡` label suffix for cached nodes in `stepLabel()` (honest middle ground; keep status colors unchanged).
23. Timeline/Gantt tab: visually distinguish cached bars (striped fill using `--cache`).
24. `live/demo` + `viz/example`: add a cached step to showcase the badge.
25. `index.go`: O(1) cached-step lookup in `ReportIndex` if consumers ask (only with a consumer).
26. Website (website/): update features page; new dashboard screenshot showing the badge.

**Hygiene / observational (P2):**
27. Benchmark `MarkCached` hot-path overhead (mutex acquisition per hit; likely negligible — prove it).
28. Consider documenting in plugin.go Config docs: interplay of disabled auditor + steps that call MarkCached unconditionally.
29. STABILITY.md: confirm additive-only minor bump semantics for the new API (should be a no-op note).
30. Docs-health pass over TODO_LIST/FEATURES after the release.
31. Count-method note or script for test metrics (see e.3).

(31 concrete items — the remaining headroom to 50 would be padding; stopping at real work.)

---

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. **Diagram exports:** should cached steps get a visible marker in Mermaid/DOT/D2/PlantUML node labels (`fetch ⚡cached`), or is dashboard/table/CLI-only the right surface? (Taste call: diagrams are status-colored today; adding a non-status annotation changes their visual vocabulary.)
2. **Release timing:** cut **v0.11.0 now** so BuildFlow can consume the feature immediately, or batch it with the Diff-cached-deltas work into one release? (RELEASE.md process is clear; the scheduling preference is yours.)
3. **Diff scope:** add cached deltas to `Diff()` before the release (it's the natural regression detector for cache-rate drift) or ship the current state first? (Both orders are defensible; it changes what the release advertises.)

---

## Session verification snapshot (for the record)

- `GOEXPERIMENT=jsonv2 go test -race ./...` — ok (core, incl. cmd/auditlog)
- `cd viz && GOEXPERIMENT=jsonv2 go test ./...` — ok
- `cd live && GOEXPERIMENT=jsonv2 go test ./...` — ok
- `go vet` × 3 modules — ok
- `golangci-lint run` × 3 modules — 0 issues each
- Schema regenerated; tree clean (daemon-committed @ `7745fc6`)

_Arte in Aeternum_

---

## Update 2026-09-11 (follow-up session): CLOSED OUT

All P0 items and the three open questions are resolved; **v0.11.0 is released**.

- **Q1 (diagrams): YES** — `stepLabel()` appends `⚡cached` on all diagram formats + both trees (label-only; status colors unchanged; cached is not a status). Pinned by `TestDiagram_CachedMarker` across 6 formats.
- **Q3 (Diff scope): YES** — `Diff()` gained `CachedStepCountDelta`, `CachedStepsAdded`/`Removed`, `StepDiff.Cached`; CLI `diff` prints cached-delta lines; new property test `TestDiff_CachedAntiSymmetry` (+ membership/count consistency, seed 9). Generator extended + counts derived from steps.
- **Q2 (release): cut immediately after both** — v0.11.0 released per RELEASE.md: three tags at b35eff2, pushed, `go mod tidy -e` ×2, standalone + `nix run .#check` green post-push, GitHub Release with notes + 4 demo binaries + checksums, `go get` verified for all three modules in clean dirs, CI green on master (d82407e), core pkg.go.dev page rendered with the new API (viz/live pages follow crawler propagation).
- **Dashboards**: `Cached` stat card (`--cache` token), `Cached only` filter chip (aria-pressed pattern, `data-cached` row flag), live fallback count from `state.steps` — in BOTH viz and live; pinned by extended honesty tests (10 assertions viz, 12 patterns live).
- **Docs**: AGENTS.md (cached.go file-map entry, Cached gotcha, nlreturn contradiction FIXED — commit fcddcaf re-enabled it, the old note was the lie, columns 10→12, tests recounted 570 = core 262 / viz 234 / live 74 with the rg method, coverage 95.3/93.4/95.9), MIGRATION.md v0.11.0 section, README Cache-Hit Attribution section + TOC, CHANGELOG 0.11.0 section, FEATURES.md updates.
- **Demos**: viz example + live demo both gained a cached detect step (`MarkCached` from the step body, honest summary lines).
- **BuildFlow wired**: deps v0.10.0→v0.11.0 (+ `go work vendor` + `update-vendor-hash`), `MarkCached(ctx)` in `detectWithCache`'s cache-hit branch (check + `slog.Debug`, no nolint — BuildFlow's erraudit gate runs `--no-suppress`, see BuildFlow gotcha #162), e2e `TestResultCache_CachedAttribution` (cold run: 0 cached; warm run: count>0, all cached steps succeeded, JSON round-trip consistent). Full execution suite -race green; erraudit gate 0 violations ×32 modules.
- **Gates**: erraudit 0 ×3 modules; art-dupl 0 clone groups at -t 3..30; workspace + standalone vet/test/lint/govulncheck all green.
- **Lesson learned (release process)**: Go 1.26.7 resolves the required core version from the proxy even in workspace mode — an unpublished require in viz/live go.mod breaks ALL local builds, so the release-skill Phase 4 "workspace verification after bump" no longer works; verify first, bump, then tag+push fast. (The skill's own gotcha table hinted at this but the Phase 4 text is now stale.)
- Not done (deliberately, P2): timeline/Gantt striped cached bars, `ReportIndex` cached lookup, website/ refresh, OTel span attribute, BuildFlow run-summary surfacing next to CacheStats. Found but not fixed (pre-existing, out of scope): BuildFlow `live_shell.go:89` stale `//nolint:exhaustruct` (linter is now `exhaustruct_v5`).
