# PRO/CONTRA: Adopting go-error-family in go-workflow-auditlog

**Date:** 2026-06-22 · **Decision needed:** Should auditlog adopt `github.com/larsartmann/go-error-family` for structured error classification?

---

## Executive Summary

**Verdict: ADOPT via registration — low cost, high optionality, correct timing.**

auditlog currently has 9 public sentinel errors with no behavioral metadata. go-error-family provides a behavioral taxonomy (Family) that maps naturally onto these errors. The registration approach (`RegisterClassification`) adds classification without changing the public API, costing one zero-dependency import. The strongest argument against is over-engineering risk — but the taxonomy fits cleanly and the ALPHA status means now is the right time.

| Factor | Assessment |
|--------|-----------|
| Go version compatibility | Identical (both `go 1.26.3`) |
| Dependency cost | Root module = stdlib only in production code |
| Breaking change risk | Zero (registration approach) |
| Taxonomy fit | Natural — validation → Rejection/Corruption, I/O → Transient |
| Timing | Correct — auditlog is ALPHA (v0.4.0), pre-stability |
| Effort | ~1 hour (registration + tests + AGENTS.md update) |

---

## Current State: auditlog Error Handling

### What exists today

- **9 public sentinels** + **2 private sentinels**, all `errors.New`
- **3 error patterns**: direct sentinel return, `fmt.Errorf("%w")` wrapping, plain context wrapping (no sentinel)
- **No custom error types** — no structs implementing `error`, no `errors.As` usage anywhere
- **~30+ error return paths** total, but only 9 have matchable identity
- **I/O/export errors have no sentinels** — `fmt.Errorf("render mermaid diagram: %w", err)` is unmatchable

### What's missing

| Gap | Impact |
|-----|--------|
| No retry hint on any error | Consumers can't decide whether to retry `LoadReport` failures |
| No machine-readable error code | Consumers must string-match or `errors.Is` each sentinel individually |
| No structured context | Error details are baked into the message string via `fmt.Errorf` verbs |
| No severity/priority signal | All errors look the same to the caller |
| I/O errors are opaque | `LoadReport`, all `Export*`, all `Write*` errors are untyped wrappers |

---

## What go-error-family Provides

### Core concept: Family

Five behavioral categories, each carrying actionable metadata:

| Family | Retry? | Exit code | HTTP | Meaning |
|--------|--------|-----------|------|---------|
| Rejection | No | 64 | 400 | Caller's fault — bad input |
| Conflict | No | 66 | 409 | State conflict — concurrent modification |
| Transient | Yes | 75 | 503 | Temporary failure — retry with backoff |
| Corruption | No | 70 | 500 | Data integrity violation |
| Infrastructure | No | 76 | 503 | System-level failure — ops needed |

### Key capabilities for auditlog

1. **`Classify(err) Family`** — universal classification from any error
2. **`IsRetryable(err) bool`** — one-call retry decision
3. **`ExitCode(err) int`** — BSD sysexits.h mapping
4. **`Error.JSON()`** — canonical JSON encoding `{"family","code","message","context","retryable"}`
5. **`RegisterClassification(sentinel, family)`** — attach Family to existing sentinels without changing them
6. **Four structural interfaces** — `Coded`, `Classified`, `Contextual`, `Retryable` — satisfy what you need, no base type required
7. **Zero-allocation hot paths** — Classify ~9-30ns, 0 allocs

---

## PRO Arguments

### 1. Zero-breakage registration path

The lowest-friction adoption: call `RegisterClassification` for each sentinel. Existing `errors.Is` checks continue working unchanged. No public API change. No breaking change for consumers. One import, one `init()` (or explicit registration function), done.

```go
// classify.go
func init() {
    errorfamily.RegisterClassifications(map[error]errorfamily.Family{
        ErrEventCountMismatch: errorfamily.Corruption,
        ErrStepCountMismatch:  errorfamily.Corruption,
        ErrStatusDrift:         errorfamily.Corruption,
        ErrCountMismatch:       errorfamily.Corruption,
        ErrEmpty:               errorfamily.Rejection,
        ErrNoEvents:            errorfamily.Rejection,
        ErrOversizedLine:       errorfamily.Rejection,
        ErrWorkflowIDPathSep:   errorfamily.Rejection,
        ErrReplayNoEvents:      errorfamily.Rejection,
    })
}
```

### 2. Natural taxonomy fit

auditlog's errors map cleanly onto the Family taxonomy with zero ambiguity:

| Error | Family | Reasoning |
|-------|--------|-----------|
| `ErrEventCountMismatch` | Corruption | Internal data integrity violation — the report is structurally invalid |
| `ErrStepCountMismatch` | Corruption | Same — structural invalidity |
| `ErrStatusDrift` | Corruption | Stored state disagrees with derived state |
| `ErrCountMismatch` | Corruption | Aggregate disagrees with actual count |
| `ErrEmpty` | Rejection | Caller sent empty input — caller's fault |
| `ErrNoEvents` | Rejection | Caller sent input with no events — caller's fault |
| `ErrOversizedLine` | Rejection | Input exceeds size limit — caller's fault |
| `ErrWorkflowIDPathSep` | Rejection | Config validation failure — caller's fault |
| `ErrReplayNoEvents` | Rejection | No events to replay — caller's fault |

No error requires a stretch or judgment call. The split is: **data integrity → Corruption, bad caller input → Rejection**.

### 3. Consumers inherit classification for free

A consumer calling `auditlog.LoadReport(path)` gets a `Corruption` or `Transient` error without importing error-family themselves. They can call `errorfamily.IsRetryable(err)` or `errorfamily.ExitCode(err)` on an auditlog error and get the correct answer. This is the "share the protocol" philosophy — the library owns its error semantics, the consumer benefits.

### 4. Retry decisions become programmable

Currently, if `LoadReport` fails with a transient I/O error, the consumer has no way to distinguish "file not found" (don't retry) from "disk temporarily unavailable" (retry). With Family classification on I/O wrappers:

```go
// Before (current)
err := auditlog.LoadReport(path)
// Is this retryable? No idea — it's just fmt.Errorf("open report: %w", err)

// After (with Family on I/O errors)
err := auditlog.LoadReport(path)
if errorfamily.IsRetryable(err) {
    // Back off and retry
}
```

### 5. Philosophical alignment with AGENTS.md

> "Strong types over runtime checks — Make impossible states unrepresentable"
> "Naming Is Architecture — every name is a contract"

Family classification is strong typing of errors. `Corruption` vs `Rejection` is a contract that tells the caller exactly what category of failure occurred, replacing string-guessing with type matching. This is the same principle auditlog already applies to `StepStatus` (typed enum vs string).

### 6. Correct timing — ALPHA means experiments are cheap

auditlog is v0.4.0 ALPHA. No stability commitment exists. If Family classification doesn't add value, removing it is trivial (delete `classify.go`, remove import). Post-1.0, this decision carries migration cost. Now it carries none.

### 7. Same maintainer, same ecosystem

Both libraries are `github.com/larsartmann/*`, both use Go 1.26.3, both follow identical conventions (stdlib testing, table-driven, zero-dependency philosophy). The integration risk is minimal — if something breaks, the same person fixes both sides.

### 8. Structured context replaces string interpolation

Current approach bakes diagnostic data into the message string:
```go
return fmt.Errorf("%w: got %d, want %d", ErrEventCountMismatch, r.EventCount, len(r.Events))
```

With Family's `WithContext`, context is machine-readable:
```go
return errorfamily.WrapCorruption(ErrEventCountMismatch, "event_count_mismatch",
    "event_count does not match len(events)").
    WithContext("got", strconv.Itoa(r.EventCount)).
    WithContext("want", strconv.Itoa(len(r.Events)))
```

Consumers can extract `"got"` and `"want"` programmatically instead of regex-parsing the error message.

### 9. JSON error serialization for the report pipeline

`Error.JSON()` produces `{"family":"corruption","code":"event_count_mismatch","message":"...","context":{...},"retryable":false}`. This could enhance the audit report format — step errors (currently stringified to `*string`) could carry structured classification metadata in the JSON/NDJSON export.

### 10. Registration enables consumer-side custom registries

If a consumer wants a different classification (e.g., they want `ErrOversizedLine` to be `Transient` for their use case), they can use a cloned `Registry` and override. The library provides defaults; the consumer retains control.

---

## CONTRA Arguments

### 1. Library vs application boundary mismatch

go-error-family's headline features — exit codes, HTTP status mapping, CLI handler (`HandleError`), diagnostic runner — live at the **application** boundary. auditlog is a **library**. It doesn't exit processes, serve HTTP, or run diagnostics. The Family metadata is metadata *for the consumer*, which means auditlog is doing work the consumer could do themselves.

**Counter:** The library knows its error semantics better than any consumer. Only auditlog knows `ErrStatusDrift` is corruption, not a transient retry. Delegating classification to consumers guarantees misclassification.

### 2. go-error-family is pre-1.0 (v0.5.0)

No API stability guarantee. A breaking change in error-family (e.g., `Family` enum renumbering, `Is` semantics change) would cascade into auditlog. The replace directives in error-family's go.mod (for unpublished submodules) signal ongoing structural churn.

**Counter:** Both libraries are pre-1.0 and by the same author. Breaking changes would be coordinated. And the registration approach is the thinnest possible coupling — remove the import, nothing breaks.

### 3. Over-engineering for 9 error conditions

auditlog has 9 public sentinels. A 5-family taxonomy with 4 interfaces, a Registry, classification functions, and message templates is arguably more ceremony than 9 errors warrant. The current `errors.Is` approach is simple, well-tested (195 tests), and has zero reported issues.

**Counter:** The registration approach adds ~10 lines of code (`classify.go`). No new concepts enter the codebase — just a mapping table. The ceremony is in error-family, not in auditlog.

### 4. Incomplete coverage creates inconsistency

Only 9 of ~30+ error return paths have sentinels. The rest are `fmt.Errorf("render X: %w", err)` with no identity. Adopting Family for only the 9 sentinels means most errors remain unclassified — the taxonomy covers validation/parse errors but ignores the majority of I/O/export errors. This is worse than no taxonomy, because it creates a false impression of completeness.

**Counter:** This is an argument for *full* adoption (wrap I/O errors too), not against adoption. The current state already has this inconsistency — sentinels exist for some paths, not others.

### 5. Global state mutation on import

If registration happens in `init()`, importing auditlog mutates error-family's `DefaultRegistry` global state. This is a Go anti-pattern — side effects on import. It also means consumers using a custom `Registry` (not `DefaultRegistry`) don't get the classification.

**Counter:** Expose an explicit `RegisterClassifications(*Registry)` function instead. Let consumers opt in. Or use the package-level `DefaultRegistry` (which is the common case).

### 6. `errors.Is` semantics change for full adoption

If auditlog eventually replaces sentinels with `*errorfamily.Error`, the `Is` method matches on code+family, not sentinel identity. `errors.Is(err, auditlog.ErrEventCountMismatch)` stops working — consumers must match on code strings or use `errorfamily.Coded` interface. This is a breaking change for the full-adoption path.

**Counter:** The registration approach avoids this entirely. Sentinels stay as-is; classification is additive metadata. Full replacement is a future decision, not a prerequisite.

### 7. Learning curve for contributors

Contributors need to understand Family classification, which Family each error belongs to, and when to use `RegisterClassification` vs returning `*errorfamily.Error` directly. The current "just return a sentinel" approach has zero learning curve.

**Counter:** The mapping table is documented. New errors get a one-line addition to the registration map. The concept is simple: "is this the caller's fault (Rejection) or a data problem (Corruption)?"

### 8. The diagnose submodule dependency in go.mod

error-family's root go.mod has `require diagnose v0.1.0` with a local replace directive. While the root package doesn't import diagnose in production code (verified — references are comments only), the go.mod entry exists and `go mod tidy` in auditlog would need to resolve it. The diagnose submodule IS published (`diagnose/v0.1.0` tag exists), so this resolves cleanly, but it adds a transitive module to the dependency graph.

**Counter:** Go doesn't download modules that aren't imported. The go.mod entry is metadata only. `go mod tidy` in auditlog would include it in go.sum but never build it.

### 9. Error-to-Family mapping adds a label, not insight

Calling `ErrEventCountMismatch` "Corruption" is correct but not enriching — anyone reading the sentinel name already knows it's a data integrity issue. The Family label duplicates information already encoded in the error name.

**Counter:** The value isn't the label for humans — it's the machine-readable signal. `Classify(err).IsRetryable()` is a one-call decision; reading the error name requires a human. The label enables programmatic branching, not just documentation.

### 10. Feature overlap with existing domain error concepts

auditlog already has domain-specific error vocabulary: `StepStatus.IsError()`, `StepInfo.HasError()`, `Event.HasError()`, `StepInfo.DeriveStatus()`. These are orthogonal to Family classification but create conceptual overlap — "error" means different things in different parts of the codebase. Adding Family introduces a third error vocabulary.

**Counter:** No overlap exists. Domain error concepts (`HasError`, `DeriveStatus`) describe *workflow step outcomes*. Family classification describes *library operation failures*. They operate on different error populations and answer different questions.

---

## Adoption Strategies

### Strategy A: Registration (recommended)

**Approach:** Keep all sentinels unchanged. Add `classify.go` with `RegisterClassification` calls.

| Aspect | Rating |
|--------|--------|
| Breaking changes | None |
| Lines of code | ~15 |
| Public API impact | None |
| Consumer benefit | `Classify()`, `IsRetryable()`, `ExitCode()` work on auditlog errors |
| Reversibility | Delete one file |
| Risk | Minimal — side-effect-on-import is the only concern |

**Implementation:**
```go
// classify.go
package auditlog

import "github.com/larsartmann/go-error-family"

func init() {
    errorfamily.RegisterClassifications(map[error]errorfamily.Family{
        ErrEventCountMismatch: errorfamily.Corruption,
        ErrStepCountMismatch:  errorfamily.Corruption,
        ErrStatusDrift:        errorfamily.Corruption,
        ErrCountMismatch:      errorfamily.Corruption,
        ErrEmpty:              errorfamily.Rejection,
        ErrNoEvents:           errorfamily.Rejection,
        ErrOversizedLine:      errorfamily.Rejection,
        ErrWorkflowIDPathSep:  errorfamily.Rejection,
        ErrReplayNoEvents:     errorfamily.Rejection,
    })
}
```

### Strategy B: Full replacement

**Approach:** Replace sentinels with `*errorfamily.Error`. All error returns become `family.NewCorruption(...)` or `family.NewRejection(...)`.

| Aspect | Rating |
|--------|--------|
| Breaking changes | Yes — `errors.Is(err, ErrX)` semantics change |
| Lines of code | ~50 (rewrite error definitions + all return sites) |
| Public API impact | Errors change type; consumers must adapt |
| Consumer benefit | Full structured errors with context, JSON, codes |
| Reversibility | Difficult — touches every error return |
| Risk | Moderate — `errors.Is` breakage, test rewrites |

### Strategy C: Consumer-side classification only

**Approach:** auditlog changes nothing. Document recommended Family mappings in AGENTS.md. Consumers register classifications in their own code.

| Aspect | Rating |
|--------|--------|
| Breaking changes | None |
| Lines of code | 0 |
| Public API impact | None |
| Consumer benefit | Only for consumers who read the docs and register manually |
| Reversibility | N/A (nothing to reverse) |
| Risk | None — but minimal value |

### Strategy D: Do not adopt

**Approach:** Stay with plain sentinels. Document the status quo.

| Aspect | Rating |
|--------|--------|
| Breaking changes | None |
| Consumer benefit | None |
| Reversibility | N/A |
| Risk | None — but misses the opportunity to add behavioral metadata while it's cheap |

---

## Secondary Improvement: I/O Error Classification

Regardless of strategy, the biggest error-handling gap is the ~20+ un-sentineled I/O/export errors. These are currently:

```go
return fmt.Errorf("open report %q: %w", path, err)        // loader.go
return fmt.Errorf("render mermaid diagram: %w", err)       // mermaid.go
return fmt.Errorf("create temp file in %q: %w", dir, err)  // plugin.go
```

With Family classification:

```go
return errorfamily.WrapTransient(err, "report.load_failed",
    fmt.Sprintf("open report %q", path))

return errorfamily.WrapInfrastructure(err, "export.render_failed",
    "render mermaid diagram")
```

This is where Family adds the most value: **retry decisions on I/O failures**. But it's also the most work — touching every error return in the export pipeline.

**Recommendation:** Do this as a follow-up after Strategy A, not as part of the initial adoption.

---

## Cost Estimate

| Task | Effort | Files |
|------|--------|-------|
| Strategy A: Registration | ~30 min | `classify.go` (new), `go.mod`, `classify_test.go` (new) |
| Update AGENTS.md | ~15 min | `AGENTS.md` |
| Update FEATURES.md | ~10 min | `FEATURES.md` |
| Tests: verify Classify() per sentinel | ~30 min | `classify_test.go` |
| **Total (Strategy A)** | **~1.5 hours** | 3-4 files |
| Strategy B: Full replacement | ~4-6 hours | ~10 files + all tests |
| I/O error classification (follow-up) | ~3-4 hours | ~8 files |

---

## Recommendation

**Adopt Strategy A (registration) now.** Reasons:

1. **Cost is near-zero** — one file, one import, no breaking changes
2. **Value is immediate** — consumers get `IsRetryable()` and `Classify()` on all 9 public errors
3. **Reversibility is total** — delete `classify.go` and the dependency is gone
4. **Timing is correct** — ALPHA status means experimentation is free; post-1.0 it costs
5. **Path to deeper adoption is open** — Strategy A doesn't preclude Strategy B or I/O classification later

**Do NOT adopt Strategy B (full replacement) now.** The breaking `errors.Is` semantics and test rewrite cost aren't justified when the registration approach delivers 80% of the value at 5% of the cost.

**Do pursue I/O error classification as a follow-up** if the registration proves valuable. This is where Family classification has the highest impact (retry decisions on transient failures).

### Decision checklist

- [ ] Confirm go-error-family v0.5.0 API stability is acceptable (same author — likely yes)
- [ ] Decide: `init()` auto-registration vs explicit `RegisterClassifications()` export function
- [ ] Verify `go mod tidy` resolves cleanly with the new dependency
- [ ] Add tests: `Classify(ErrX) == expectedFamily` for all 9 sentinels
- [ ] Update AGENTS.md with the classification mapping table
- [ ] Consider: should auditlog expose `Classifications() map[error]Family` for consumer-side custom registries?

---

## Appendix: Full Error Inventory with Proposed Classification

| # | Error | File | Current Pattern | Proposed Family | Strategy |
|---|-------|------|-----------------|-----------------|----------|
| 1 | `ErrEventCountMismatch` | report.go:22 | sentinel + `%w` wrap | Corruption | A (register) |
| 2 | `ErrStepCountMismatch` | report.go:25 | sentinel + `%w` wrap | Corruption | A (register) |
| 3 | `ErrStatusDrift` | report.go:29 | sentinel + `%w` wrap | Corruption | A (register) |
| 4 | `ErrCountMismatch` | report.go:29 | sentinel + `%w` wrap | Corruption | A (register) |
| 5 | `ErrEmpty` | ndjson.go:14 | direct return | Rejection | A (register) |
| 6 | `ErrNoEvents` | ndjson.go:15 | direct return | Rejection | A (register) |
| 7 | `ErrOversizedLine` | ndjson.go:16 | sentinel + `%w` wrap | Rejection | A (register) |
| 8 | `ErrWorkflowIDPathSep` | plugin.go:50 | sentinel + `%w` wrap | Rejection | A (register) |
| 9 | `ErrReplayNoEvents` | replay.go:10 | direct return | Rejection | A (register) |
| 10 | `errUnknownEventType` | ndjson.go:18 | private sentinel | Rejection | A (register, private) |
| 11 | `errUnknownPhase` | ndjson.go:19 | private sentinel | Rejection | A (register, private) |
| 12+ | I/O/export errors | loader.go, plugin.go, all diagram files | `fmt.Errorf` only | Transient / Infrastructure | Follow-up |
