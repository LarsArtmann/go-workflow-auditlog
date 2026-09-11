# Status Report — Error Management / Sentinel Classification Overhaul

**Date:** 2026-09-11 07:55 CEST
**Repo:** go-workflow-auditlog (core + viz + live, all at master)
**Session scope:** One session, triggered by an erraudit violation report (86 findings) pasted by the user with the directive "superb error management #DDD".
**Trees touched:** 18 files (9 core `.go`, 1 core test, 2 `cmd/auditlog`, 2 `live/`, 2 `viz/`, 1 `testhelpers/`, 1 `AGENTS.md`). Working tree is clean — the auto-commit daemon captured all work (commits `4361afd`, `8758c7c`, `e29a05f`).

**Headline numbers:** erraudit violations 86 → **0** (and the honest default-flag baseline 5 → **0**) across all three modules · golangci-lint 0 issues × 3 modules · `nix run .#check` (vet + race + lint + govulncheck, all modules): **All checks passed** · 1 pre-existing broken test (CSS token drift) fixed on sight.

---

## The one-paragraph truth

The pasted report's premise — "project enforces samber/oops" — was **false** (verified: oops flags are opt-in; no oops anywhere in this repo or any sibling larsartmann lib; `how-to-golang` policy prescribes cockroachdb/errors). Instead of a wrong wholesale rewrite, the session adopted the ecosystem's *actual* intended design: every sentinel **owned** by auditlog now carries its go-error-family classification **intrinsically** (`*errorfamily.Error`, stable `auditlog.*` codes), `errors.Is` contracts fully preserved and tested, the registry demoted to an honest back-compat shim, three genuinely misclassified sentinels fixed, real swallowed close errors surfaced, and every remaining discard explicitly suppressed with a reason. All rejected findings are documented with rationale in AGENTS.md.

---

## a) FULLY DONE

**Verification (before any code):**
- ✅ Verified the oops-enforcement claim is false: `erraudit --help` shows `--enforce-samber-oops` / `--enforce-generic-return` / `--enforce-deferred-close` are opt-in flags; the 86 findings required those flags. Default-flag baseline established: 5 violations.
- ✅ Verified zero samber/oops usage in go-atomic-write, go-ndjson, go-sse, go-output, and this repo's go.mods. `go-error-family/bridge` exists as the *optional* oops adapter — confirming oops is not the ecosystem default.
- ✅ Read `go-error-family` source: `Error()` format (`[family:code] message`), `Is()` matches on **code+family**, `Unwrap` → cause, `Classify` cascade (Classified interface beats registry), `NewRegistry()` exists. This determined the whole design.
- ✅ Grepped all 16 named errors; established ownership split: 15 owned sentinels vs 3 re-exported go-ndjson sentinels (`ErrEmpty`, `ErrNoEvents`, `ErrOversizedLine` — registry classification is *correct* for those, matching "errors you don't own").
- ✅ Confirmed no test or doc asserts exact sentinel message strings → migration low-risk.

**Core design implementation (the DDD move):**
- ✅ 15 owned sentinels migrated to family constructors with dot-namespaced codes: `ErrEventCountMismatch`→Corruption/`auditlog.event_count_mismatch`, `ErrStepCountMismatch`, `ErrStatusDrift`, `ErrCountMismatch`, `ErrRenderFailed`, `ErrExportWriteFailed`, `ErrWorkflowIDPathSep`, `ErrReplayNoEvents`, `ErrReportLoadFailed`→Transient, `ErrFileExists`→Rejection (via `errorfamily.Wrap(ErrExportWriteFailed, …)` so the parent chain survives), `ErrMigrationEmptyInput`, `ErrMigrationMissingVersion`, plus private `errUnknownEventType`, `errUnknownPhase`, `errNilStreamCallback`.
- ✅ `classify.go` rewritten honestly: registration is now documented as a back-compat shim; only the three go-ndjson re-exports actually need it. `init()` into `DefaultRegistry` kept.
- ✅ Classification gap fixed: `ErrFileExists`, `ErrMigrationEmptyInput`, `ErrMigrationMissingVersion` added to `ErrorClassifications()` (previously unregistered; the migration pair was misclassified as retryable Transient via the fail-open default).
- ✅ `errors.Is` contract preserved end-to-end: identity, single wrap, nested double-wrap, `ErrFileExists → ErrExportWriteFailed` cause chain — all under test.
- ✅ New tests: `TestClassify_IntrinsicClassificationWithoutRegistry` (empty registry proves intrinsic classification; pins exactly 3 non-intrinsic sentinels), `TestErrorClassifications_CodesUniquePerFamily` (guards the `Is`-by-code+family collision footgun), `TestClassify_ErrFileExistsIsRejectionWithWriteChain`, `TestClassify_MigrationSentinelsAreRejection`; `allPublicSentinels` extended 12→15; +3 rows in the family/exit-code table.

**Real defect fixes (the "ignored" findings):**
- ✅ `cmd/auditlog convert`: output-file Close error now propagated via named return + `errors.Join` (buffered-flush data-loss risk, was silently dropped).
- ✅ `live.Server.Shutdown`: failed subscriber-buffer drain is now surfaced to the caller (`drain subscriber buffers: …`) instead of `_ =` discarded.
- ✅ Read-only input Close idioms: `loader.go` and `cmd/auditlog load.go` now use bare `defer f.Close()` / `defer closer.Close()`.
- ✅ `viz/example`: log-and-return swallow → `log.Fatalf` (observable exit status).
- ✅ Every legitimate discard suppressed with an inline reason (`//nolint:erraudit // …`): 4 client-disconnect writes + Send + Stream Close in `live/server.go`, hub-assigned-ID defensive parse skip in `live/replay.go`, infallible `crypto/rand.Read` in `runid.go` (Go 1.24+), fixture-expected failure in `testhelpers.RunWorkflow`.

**Hygiene & docs:**
- ✅ erraudit: 0 violations in core, viz, live (default flags). `nix run .#check`: all green.
- ✅ Pre-existing `TestDesignTokensInSync` failure fixed (`viz/dashboard.css` font stacks re-wrapped to match the canonical `DesignTokensCSS` const) — drift was introduced by an earlier daemon commit, not this session.
- ✅ AGENTS.md: rewrote the "Error classification" bullet (now documents intrinsic classification, full mapping with codes) and added a new "Error management policy (erraudit-clean)" bullet documenting the wrapping idiom, the rejected flags, and every suppression.

---

## b) PARTIALLY DONE

1. **`live/hub.go:227`** — `fmt.Errorf("drain: %w", ctx.Err())` is still an uncategorized error (no sentinel, no family, no code). Spotted during the pass, deliberately deferred; inconsistent with the now-intrinsic architecture.
2. **Error() format change is user-visible but undocumented** — sentinel messages gained the `[family:code] ` prefix (CLI output changes shape). Not reflected in `docs/MIGRATION.md` or a CHANGELOG entry.
3. **Rejection of the strict flags is documented in exactly one place** (AGENTS.md policy bullet). No decision-log/ADR artifact; no CI wiring that would *enforce* the default-flag cleanliness so it doesn't rot.
4. **Per-module standalone verification** — all three modules passed vet/lint/test/race via `nix run .#check`, but the explicit `GOWORK=off` standalone test commands from the AGENTS.md table were not run individually this session.
5. **`erraudit nolint-audit` discrepancy** — it reports "No //nolint:erraudit directives found" while the analyzer demonstrably honors the 9 directives I added. Noticed, not root-caused (possibly comment-association differences in its go/parser pass).
6. **Suppression catalog** — the 9 suppressions each carry a local reason, but there is no single inventory listing them for periodic re-audit (staleness detection exists as `erraudit nolint-audit`, see #5).

---

## c) NOT STARTED

- **CHANGELOG.md** entry for the error-model change (intrinsic families, codes, `[family:code]` prefix, 3 reclassified sentinels).
- **Release decision** — the public surface changed: sentinel vars are now `*errorfamily.Error` (visible in godoc; `errors.AsType[*errorfamily.Error]` on library errors now succeeds where it previously failed). No version bump, no tags, no RELEASE.md process started.
- **CI gate for erraudit** — no workflow change; default-flag cleanliness is currently enforced by nothing.
- **CLI family-aware exit codes** — `main.go` still does a blanket `os.Exit(1)`; `errorfamily.ExitCode(err)` (65/69/75/1) is available for free and unused. Usage errors also still exit 1, not 2.
- **README error-handling section** — consumers have no doc for the new codes / `errors.AsType[*errorfamily.Error]` / `Classify` usage.
- **docs-health HARVEST** — section (f) below has not been routed into `TODO_LIST.md` / `ROADMAP.md`.
- **Coverage + duplication re-measurement** — new tests were added but `coverprofile` (~95.4% claim in AGENTS.md) and `art-dupl -t 1` were not re-run this session.
- **AGENTS.md test-count staleness** — the "491 test functions" claim predates this session's additions.
- **AGENTS.md commands table** — no `erraudit` row despite it being the project's error gate.

---

## d) TOTALLY FUCKED UP (brutal honesty — nothing catastrophic, but you deserve the full list)

1. **I nearly had the wrong flagship.** The pasted report's "project enforces samber/oops" line could have driven a dependency-adding rewrite of 39 wrap sites + 16 sentinels against a policy that doesn't exist. Only the load-skill-first + verify-external-claims discipline stopped it. If I had skipped that step, this session would have shipped a wrong architecture confidently. (Not fucked up — but it was one skipped step away from fucked up.)
2. **False "all green" moment.** My first `erraudit .` run printed "Total Errors Found: 0" — because the `.` argument analyzed nothing. I caught it by re-running `./...`, but for a moment the tool output said everything was fine and it was lying by not running. Lesson recorded: verify the tool actually analyzed code before believing zero.
3. **Edit-before-read failure.** First `multiedit` on `plugin.go` was rejected because I hadn't used the View tool (only bash `cat`). Wasted round trip; violated my own rule #1.
4. **Formatter fight I started.** My long trailing nolint reason on `testhelpers.go:412` exceeded golines' 120 columns; the auto-fix wrapped it into an ugly 3-line call. Caught it in the diff, fixed by shortening the reason — but I should have counted the columns before writing the line.
5. **errcheck exclusion semantics assumed, not checked.** I assumed `defer stream.Close()` would pass because `(io.Closer).Close` is excluded in core's errcheck config — it didn't in live. Empirically discovered via lint failure, then reverted to the explicit-ignore + nolint form. I also never confirmed *why* (live's config vs errcheck's interface matching) — unverified root cause left behind.
6. **A behavioral surface change shipped without a version gate.** Sentinel concrete type changed to `*errorfamily.Error`. Consumers doing `errors.AsType[*errorfamily.Error]` or type-switching on sentinels get different results today than yesterday. `errors.Is` is preserved and tested, but the type-level surface moved under an ALPHA version with zero release notes. That is the single most defensible criticism of this session.
7. **Unresolved oddity I let stand:** `nolint-audit` not seeing the directives it clearly honors (see b6). I chose not to chase it — but "it works, don't know why" is debt.

**What was NOT fucked up:** no data loss, no broken API matching, no test regressions (full suite + race green), no scope creep into unrelated refactors, historical status docs deliberately left untouched (point-in-time snapshots).

---

## e) WHAT WE SHOULD IMPROVE

1. **Gate what you claim.** "0 violations" should be a CI job, not a session memory.
2. **Treat error-model changes as API changes.** Sentinel type + message format ⇒ CHANGELOG/MIGRATION + release discipline, even in ALPHA.
3. **Design away impossible error paths instead of suppressing them.** The ring buffer should store numeric sequence numbers, making the parse-skip branch (and its nolint) structurally unnecessary — suppression was the pragmatic choice, elimination is the right one.
4. **Finish the error taxonomy.** `live/hub.go` drain error and the two bare `fmt.Errorf` paths in `replay.go`/`migration.go` (validation/unmarshal without sentinels) are the remaining uncategorized errors in owned code.
5. **Use the family machinery you already have.** The CLI's exit codes and the documented per-family BSD codes are begging to be wired together.
6. **Centralize suppression governance** — one inventory + the (currently confusing) `nolint-audit` staleness command.
7. **Process fixes adopted going forward:** view-before-edit without exception; count formatter constraints (120 cols) when writing trailing directives; after any "0 findings" result, confirm the tool actually traversed the code.

---

## f) Up to 50 things we should get done next (brainstorm — impact-ordered within tiers; most of tier 3 is ROADMAP fuel)

**Tier 1 — high impact, low effort:**
1. CI job: `erraudit ./...` (default flags) in all three modules; fail on any violation.
2. CHANGELOG.md entry for the sentinel classification overhaul.
3. docs/MIGRATION.md note: `[family:code]` message prefix + sentinel concrete type now `*errorfamily.Error`.
4. AGENTS.md commands table: add `erraudit` row(s) for all modules.
5. AGENTS.md: refresh test-count and coverage claims after a fresh `coverprofile` run.
6. Wire CLI exit codes to `errorfamily.ExitCode(err)` (Corruption 65 / Infrastructure 69 / Transient 75 / Rejection 1).
7. CLI usage errors → exit 2 (currently 1, indistinguishable from real failures).
8. live: give the hub drain error a sentinel (family + code) — `live/hub.go:227`.
9. replay.go: sentinel for "replayed report failed validation" (Corruption family fits).
10. migration.go: sentinel for the bare `unmarshal report: %w` path (Rejection).
11. Route this list through docs-health HARVEST into TODO_LIST.md / ROADMAP.md.
12. README: "Error handling" section — code table, `errors.Is` and `errors.AsType[*errorfamily.Error]` examples.

**Tier 2 — solidify:**
13. Release the change (RELEASE.md process, three tags) so the API shift is versioned, not ambient.
14. Decision-log/ADR: why not samber/oops, why not cockroachdb/errors, why stdlib dual-wrap.
15. Root-cause the `nolint-audit` "no directives found" discrepancy.
16. Central suppression inventory doc; run `erraudit --no-suppress` once and reconcile.
17. Store numeric seq in `eventRingBuffer` entries; delete the parse-skip branch + its nolint.
18. Test: `live.Server.Shutdown` surfaces drain failure (expiring-context drain).
19. Test: cmd convert close-error propagation (failing closer injection).
20. Property test: arbitrary wrap chains classify stably through `Classify`/`Code`/`ExitCode`.
21. Fuzz/adversarial: two sentinels, same code different family (and vice versa) — pin `Is` semantics.
22. Explicit `GOWORK=off` standalone test runs for viz + live (AGENTS commands) in this repo's check flow.
23. Re-run `art-dupl -t 1` after the new tests; keep zero clone groups.
24. Confirm AGENTS.md coverage claims with a fresh `coverprofile=atomic` run.
25. docs-health VERIFY pass over the two new AGENTS.md error bullets (claims vs code).
26. Evaluate adopting `errorfamily.RegisterStdlibDefaults` (context/sql/os sentinels) for richer consumer classification of wrapped OS errors.
27. Evaluate exporting the error-code strings as constants (consumer-side typo safety).
28. Evaluate exporting `ErrNilStreamCallback` (currently private → unmatchable by consumers).
29. Annotate the 2026-06-23 go-error-family adoption status report (docs-health ANNOTATE) with a pointer to the new intrinsic architecture.
30. Confirm JSON Schema / goldens are truly unaffected by error-string changes (expected yes; verify once).

**Tier 3 — roadmap fuel:**
31. Error-code registry page for the docs website (auto-generated from `ErrorClassifications()`).
32. `oops`-bridge consumer guide (errorfamily/bridge exists; document the interop path for oops-using consumers).
33. Benchmark: wrapping overhead `fmt.Errorf` dual-wrap vs `errorfamily.Wrapf` (data for future decisions).
34. Review all remaining `fmt.Errorf` wrap sites for context consistency (path/step/line coverage).
35. Consider `slog` integration example for `HandleError`/structured logging of classified errors.
36. Consider a `Classified`-interface example for consumers building domain error types (per go-error-family philosophy).
37. Packaging: add erraudit to `flake.nix` devShell/check (currently a `~/go/bin` binary outside Nix).
38. CI: `erraudit nolint-audit` staleness gate (after #15 resolves its semantics).
39. Dashboard/help modal: show family/exit-code legend for CLI errors (UX polish).
40. Audit `OnEvent`/`MultiWriter` docs to state explicitly they carry no sentinels by design (events are data).
41. Sweep gopls `stdversion` warnings noise (GOEXPERIMENT jsonv2 vs gopls) — either suppress or document.
42. gopls `unusedfunc` finding `failure_reason_test.go:346 strPtr` — remove or use.
43. gopls `bloop` findings (`b.N` → `b.Loop()`) in diff_test/stream_test — modernize benchmarks.
44. Deprecation watch: `exhaustruct` → `exhaustruct_v5` in all three `.golangci.yml` files.
45. Review whether `ErrorClassifications()` should return a cached map (per-call allocation; hot-path irrelevant, note only).
46. docs: mention `ErrFileExists` → `ErrExportWriteFailed` chain explicitly in README matching examples.
47. Consider `context.DeadlineExceeded`-style classification tests for wrapped stdlib errors flowing through auditlog paths.
48. Add `erraudit` invocation to the release pre-flight checklist (RELEASE.md).
49. Review `live` module for other silent swallows under stricter erraudit flags and either fix or explicitly reject in the policy bullet.
50. Post-release: consumer smoke test (`go get` all three modules standalone) per RELEASE.md.

---

## g) Questions I cannot figure out myself

Asked interactively after this report (also recorded here):

1. **Release policy:** the sentinel concrete type and `Error()` string format changed — do you want a release cut now (CHANGELOG + MIGRATION + three tags) so the API shift is versioned, or should ALPHA absorb it silently until the next feature release?
2. **CI policy:** should `erraudit ./...` (default flags, all three modules) become a required CI gate — the enforcement that keeps "0 violations" true — or stay a manual/local discipline?
3. **CLI behavior:** may I change the auditlog CLI's exit codes to family-based values (65/69/75/1) with usage errors on 2? That changes scripted-consumer behavior, so it needs your call.

---

*Point-in-time snapshot. Stale the moment `master` moves. Route section (f) through `docs-health` HARVEST; do not resurrect this file as living documentation.*
