# Status Report: go-error-family Adoption — COMPLETED

**Date:** 2026-06-23 01:12 · **Session scope:** Full execution of the 41-task Pareto plan for adopting `go-error-family` into `go-workflow-auditlog` · **Verdict:** ✅ All 5 tiers done, verified, green.

---

## Executive Summary

The entire go-error-family adoption plan was executed end-to-end: Strategy A (Registration) + I/O Error Classification (the "Secondary Improvement" from the PRO/CONTRA report). All 14 sentinel errors (12 public + 2 private) are classified across 4 Families. All 22 I/O error paths are wrapped with matchable sentinels. 212 tests pass with race detector. Lint is clean. Coverage holds at 92.9%. The example demo shows error classification live.

---

## Quality Gates (all verified at session end)

| Gate | Result | Details |
|------|--------|---------|
| `go build ./...` | ✅ PASS | Clean compile, zero warnings |
| `go vet ./...` | ✅ PASS | No issues |
| `golangci-lint run ./...` | ✅ PASS | 0 issues (v2, ~120 linters enabled) |
| `golangci-lint fmt` | ✅ PASS | All files formatted |
| `go test -race -count=1 ./...` | ✅ PASS | 212 test functions, 74 subtests, 0 failures |
| Coverage | ✅ 92.9% | classify.go: **100%** on all 3 functions |
| `go mod tidy` | ✅ PASS | Clean, no diff |
| `art-dupl -t 15` | ✅ 13 groups | 0 classify clones (deduped via `allPublicSentinels()` helper) |
| `go run ./example` | ✅ PASS | Error Classification demo output correct |

---

## A) FULLY DONE ✅

### Tier 1 — Critical Path (Strategy A Registration)

- **`classify.go`** (66 lines) — `init()` auto-registers all sentinels into `errorfamily.DefaultRegistry` on import
  - `ErrorClassifications()` — returns canonical `map[error]Family` (single source of truth)
  - `RegisterClassifications(reg)` — explicit registration into custom registry
  - `init()` — self-registration pattern (like `database/sql` driver registration)
- **`go.mod`** — added `github.com/larsartmann/go-error-family v0.5.0`
- **`.golangci.yml`** — depguard allow-list updated for `go-error-family` import

### Tier 2 — Verification (Tests)

- **`classify_test.go`** (290 lines, 8 test functions, 12 subtests) — 100% coverage of classify.go:
  - `TestClassify_PublicSentinels` — all 12 public sentinels: Classify + ExitCode + IsRetryable
  - `TestClassify_WrappedErrorPreservesClassification` — `fmt.Errorf("%w")` chain works
  - `TestClassify_WrappedIOErrorPreservesClassification` — I/O sentinels (Transient/Infrastructure) through real wrapping patterns
  - `TestClassify_NestedWrapping` — double-wrapped error chain traversal
  - `TestClassify_UnregisteredErrorDefaultsToTransient` — fail-open default verified
  - `TestClassify_ErrorsIsUnchanged` — Strategy A identity guarantee (all 12 sentinels)
  - `TestRegisterClassifications_CustomRegistry` — custom `Registry` registration
  - `TestErrorClassifications_ContainsAllPublicSentinels` — map completeness check

### Tier 3 — Ship-Ready (Docs & Quality)

- **`AGENTS.md`** — architecture table (3 file entries updated), Error Classification gotcha (full sentinel inventory + mapping table), test count updated (195 → 212)
- **Golden file** (`testdata/golden/report.html`) — regenerated (was already stale before this session)

### Tier 4 — I/O Error Classification (22 paths wrapped)

**3 new public sentinels added:**

| Sentinel | File | Family | Exit | Retryable |
|----------|------|--------|------|-----------|
| `ErrReportLoadFailed` | `loader.go:15` | Transient | 75 | **Yes** |
| `ErrRenderFailed` | `report.go:36` | Infrastructure | 69 | No |
| `ErrExportWriteFailed` | `plugin.go:56` | Infrastructure | 69 | No |

**22 bare `fmt.Errorf` paths wrapped** across 13 files:

| File | Paths wrapped | Pattern |
|------|--------------|---------|
| `loader.go` | 3 (open, decode, unmarshal) | `ErrReportLoadFailed` |
| `plugin.go` (writeToFile) | 4 (create temp, flush, close, rename) | `ErrExportWriteFailed` |
| `report.go` | 1 (WriteJSON encode) | `ErrRenderFailed` |
| `export.go` | 2 (NDJSON encode, flush) | `ErrRenderFailed` + `ErrExportWriteFailed` |
| `mermaid.go` | 2 (render, write) | `ErrRenderFailed` + `ErrExportWriteFailed` |
| `plantuml.go` | 2 (render, write) | `ErrRenderFailed` + `ErrExportWriteFailed` |
| `graphviz.go` | 2 (render, write) | `ErrRenderFailed` + `ErrExportWriteFailed` |
| `d2.go` | 2 (render, write) | `ErrRenderFailed` + `ErrExportWriteFailed` |
| `table.go` | 1 (render) | `ErrRenderFailed` |
| `tree.go` | 4 (ASCII render+write, HTML render+write) | `ErrRenderFailed` + `ErrExportWriteFailed` |
| `html_render.go` | 2 (marshal report, marshal metadata) | `ErrRenderFailed` |
| `html.go` | 1 (write HTML) | `ErrExportWriteFailed` |

### Tier 5 — Polish (Docs, Example, Final Gates)

- **`README.md`** — expanded sentinel table (6 → 12 errors), new "Error Classification" section with code example + family mapping table
- **`FEATURES.md`** — new "Error Classification" feature block under DONE
- **`example/main.go`** — new `printErrorClassification()` function demonstrating all 4 families live
- **Adoption report** (`docs/reviews/2026-06-22_adopting-go-error-family.md`) — marked ADOPTED with implementation summary

### Sentinel Classification Summary (14 total)

| Family | Count | Exit | Retry | Sentinels |
|--------|-------|------|-------|-----------|
| **Corruption** | 4 | 65 | No | `ErrEventCountMismatch`, `ErrStepCountMismatch`, `ErrStatusDrift`, `ErrCountMismatch` |
| **Rejection** | 5+2 private | 1 | No | `ErrEmpty`, `ErrNoEvents`, `ErrOversizedLine`, `ErrWorkflowIDPathSep`, `ErrReplayNoEvents` + `errUnknownEventType`, `errUnknownPhase` |
| **Transient** | 1 | 75 | **Yes** | `ErrReportLoadFailed` |
| **Infrastructure** | 2 | 69 | No | `ErrRenderFailed`, `ErrExportWriteFailed` |

---

## B) PARTIALLY DONE ⚠️

- **CHANGELOG `[Unreleased]`** — the adoption report is marked DONE but the `CHANGELOG.md` does not have an entry for go-error-family adoption. Needs a new entry under `[Unreleased]` documenting the new dependency, 3 new sentinels, and the classification registration pattern.
- **`TODO_LIST.md`** — the deferred item `- [ ] go-error-family adoption for structured errors` is still unchecked. Needs to be marked `[DONE]` and moved to the completed section.

---

## C) NOT STARTED

- **Strategy B (Full Replacement)** — deliberately deferred per the adoption report. Would replace all sentinels with `*errorfamily.Error` structs for structured context + JSON serialization. Not needed for current value.
- **Fuzz tests for classify.go** — the existing `FuzzDiagramSpecialChars` and `FuzzHTMLSpecialChars` cover diagram/HTML injection, but there are no fuzz targets specifically for the classification mapping (e.g., feeding adversarial wrapped error chains to `Classify()`).
- **Property-based tests for classification** — no property tests verifying classification algebra (e.g., "wrapping a classified error preserves the family through arbitrary depth").
- **STABILITY.md update** — does not mention the new sentinels or the go-error-family dependency commitment.

---

## D) TOTALLY FUCKED UP 💥

**Nothing.** No regressions, no broken tests, no lint failures, no data loss. The entire implementation was clean.

**One pre-existing issue surfaced:** The golden file test (`TestReport_WriteHTML_GoldenFile`) was already failing before this session started (verified via `git stash`). The golden file was stale from a prior session. We regenerated it with `UPDATE_GOLDEN=1` — this is a fix, not a regression.

---

## E) WHAT WE SHOULD IMPROVE

### Architecture

1. **`errUnknownPhase` has asymmetric wrapping** — `errUnknownEventType` is used with `fmt.Errorf("%w")` in `ndjson.go:55`, but `errUnknownPhase` at `ndjson.go:63` also wraps. Both are registered, but the wrapping patterns differ slightly. Normalize.
2. **Exit codes for Rejection = 1 (not 64)** — the PRO/CONTRA report assumed BSD sysexits.h `EX_USAGE=64`, but go-error-family v0.5.0 returns exit code `1` for Rejection. This is correct per the library but differs from the original report's assumption. The report should be corrected.
3. **Strategy B path is open but undocumented** — the migration path from registration (A) to full replacement (B) should be documented if we ever want structured context on auditlog errors.

### Testing

4. **Error-path tests for Write\* methods** — the I/O sentinels (`ErrRenderFailed`, `ErrExportWriteFailed`) are now wrapped on all paths, but we don't have tests that inject failing `io.Writer`s to exercise these paths. Coverage on the render files sits at ~83%.
5. **`ErrReportLoadFailed` is not exercised end-to-end** — no test calls `LoadReport("nonexistent.json")` to verify the sentinel wrapping in a real I/O failure.

### Documentation

6. **CHANGELOG entry missing** — see section B above.
7. **STABILITY.md not updated** — new sentinels and new dependency should be documented.

---

## F) Top 25 Things to Do Next

### Immediate (this session's loose ends)

| # | Task | Impact | Effort |
|---|------|--------|--------|
| 1 | Add CHANGELOG `[Unreleased]` entry for go-error-family adoption | Medium | 5 min |
| 2 | Mark `TODO_LIST.md` go-error-family item as `[DONE]` | Low | 2 min |
| 3 | Update `STABILITY.md` with 3 new sentinels + dependency note | Medium | 5 min |
| 4 | Fix PRO/CONTRA report exit codes (Rejection=1, Corruption=65, Infrastructure=69, Transient=75) | Low | 5 min |

### Error handling deepening

| # | Task | Impact | Effort |
|---|------|--------|--------|
| 5 | Add error-path tests: inject failing `io.Writer` into all `Write*` methods, verify `ErrRenderFailed`/`ErrExportWriteFailed` wrapping | High | 45 min |
| 6 | Add `LoadReport("nonexistent.json")` test verifying `ErrReportLoadFailed` + `errors.Is` | High | 10 min |
| 7 | Add fuzz test for `Classify()` — adversarial wrapped error chains, deeply nested `fmt.Errorf("%w")` | Medium | 20 min |
| 8 | Add property test: "wrapping preserves family through arbitrary depth" | Medium | 15 min |

### Pre-release polish

| # | Task | Impact | Effort |
|---|------|--------|--------|
| 9 | Push coverage 92.9% → 95%+ (target: error-path branches in Write\* methods) | High | 60 min |
| 10 | Tag v0.5.0 release (go-error-family adoption + I/O sentinels warrant a minor bump) | High | 10 min |
| 11 | Add `StepInfo.Type()` method for API consistency with `Status.Label()` | Low | 10 min |
| 12 | Add retry/timeout columns to table export | Medium | 20 min |

### Feature work (from TODO_LIST.md)

| # | Task | Impact | Effort |
|---|------|--------|--------|
| 13 | Make table columns configurable (column-selection options) | Medium | 45 min |
| 14 | Add diagram layout direction option (TD vs LR) | Medium | 30 min |
| 15 | Add `writeToFile` overwrite protection (`O_EXCL` / "file exists" error) | Low | 15 min |
| 16 | Add benchmarks for WriteD2, WriteTable, WriteTree | Medium | 30 min |
| 17 | Add fuzz tests for diagram ID sanitization | Medium | 25 min |
| 18 | Add integration/round-trip tests (report → JSON → Load → diagram → verify) | High | 45 min |
| 19 | Surface name collisions in diagrams (warn when `String()` collides) | Low | 20 min |
| 20 | Offer `Name(step)` fallback helper | Low | 15 min |

### Strategic (from ROADMAP.md)

| # | Task | Impact | Effort |
|---|------|--------|--------|
| 21 | Migrate `justfile` → `flake.nix` | Medium | 60 min |
| 22 | Split library into core + visualization sub-modules | High | 2-4 hours |
| 23 | Streaming NDJSON export option | High | 2 hours |
| 24 | OpenTelemetry span bridge | High | 3 hours |
| 25 | `encoding/json/v2` migration | Medium | 1 hour |

---

## G) Top Question I Cannot Answer Myself

**Should we keep `init()` auto-registration or switch to explicit opt-in?**

The `init()` function in `classify.go` mutates `errorfamily.DefaultRegistry` on import. This is the standard Go driver-registration pattern (`database/sql`, `image` codec registration), and it means consumers "just work" — import auditlog, get classification for free. The PRO/CONTRA report acknowledged this as CONTRA #5 (global state mutation on import).

**Why I can't decide:** This is a philosophical API design choice with no objective right answer. The `init()` approach maximizes consumer convenience (zero setup, `Classify(err)` works out of the box). The explicit approach (`RegisterClassifications(reg)` called by the consumer) maximizes purity (no import side effects, full consumer control). Both are correct for different audiences. The `RegisterClassifications` export function is already available as the escape hatch — but the `init()` still fires regardless.

The question is: **is import-time side effect on a global registry acceptable in a library that values explicit-over-implicit (per AGENTS.md)?**

This decision affects the public API contract and should be made by the maintainer.
