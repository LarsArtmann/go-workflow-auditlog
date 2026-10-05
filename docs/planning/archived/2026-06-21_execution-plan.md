# Execution Plan — 2026-06-21 Post-Review Backlog

Derived from the three status reports in `docs/status/2026-06-21*` after
verifying the claims against the actual codebase. Each task is scoped to
**≤12 minutes** and sorted by importance → impact → effort → customer-value.

## Verified codebase state (corrections to reports)

Reports made claims that are **already resolved** as of HEAD `4849d34`:

| Report claim                         | Actual state (verified)                    |
| ------------------------------------ | ------------------------------------------ |
| go-output sub-modules stuck at v0.13 | ✅ Fixed — all at v0.17.0 (commit 4849d34) |
| LSP warning on d2.go:9               | ✅ Gone (version skew fixed)               |
| No `ExportD2/Table/Tree` on Auditor  | ✅ Fixed — all exist in plugin.go          |
| `docs/DOMAIN_LANGUAGE.md` missing    | ✅ Exists (7046 bytes, Jun 18)             |
| `buildD2Graph` duplication           | ✅ Fixed — thin converter (commit 7bd0f99) |
| stepLabel/treeStepLabel split        | ✅ Unified (commit f1aa9f1)                |
| Status colors in scattered maps      | ✅ Consolidated into StepStatus.Color()    |

## Confirmed open issues (this plan addresses these)

| # | Issue                                           | Severity        | Verified location              |
| - | ----------------------------------------------- | --------------- | ------------------------------ |
~~| 1 | CHANGELOG [Unreleased] empty                    | Critical        | CHANGELOG.md:8-16              |~~ done (resolved same session / by v0.2.0 — see 2026-06-21_04-46 report)
~~| 2 | Edge direction diagrams ≠ tree                  | High bug        | diagram.go:67 vs tree.go:56    |~~ done (resolved same session / by v0.2.0 — see 2026-06-21_04-46 report)
~~| 3 | No edge-direction test assertions               | High            | tests only check "edge exists" |~~ done (resolved same session / by v0.2.0 — see 2026-06-21_04-46 report)
~~| 4 | Duration() in diff.go not report.go             | Medium cohesion | diff.go:110                    |~~ done (resolved same session / by v0.2.0 — see 2026-06-21_04-46 report)
~~| 5 | D2 title hardcoded, others none                 | Medium polish   | d2.go:50                       |~~ done (resolved same session / by v0.2.0 — see 2026-06-21_04-46 report)
~~| 6 | No Write\*String on Auditor                     | Medium API      | plugin.go                      |~~ done (resolved same session / by v0.2.0 — see 2026-06-21_04-46 report)
~~| 7 | No Export\* on WorkflowReport                   | Medium API      | report.go                      |~~ done (resolved same session / by v0.2.0 — see 2026-06-21_04-46 report)
~~| 8 | FEATURES.md / TODO_LIST.md / ROADMAP.md missing | Critical docs   | repo root                      |~~ done (resolved same session / by v0.2.0 — see 2026-06-21_04-46 report)
~~| 9 | README missing 3-duration explainer             | Low docs        | README.md                      |~~ done (resolved same session / by v0.2.0 — see 2026-06-21_04-46 report)

## Tasks — sorted by impact/effort/customer-value

### Tier P0 — Critical (execute now)

| ID | Task                                        | Impact   | Effort | Est |
| -- | ------------------------------------------- | -------- | ------ | --- |
~~| T1 | Populate CHANGELOG [Unreleased]             | Critical | Low    | 10m |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T2 | Fix diagram edge direction → execution flow | High     | Low    | 6m  |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T3 | Add edge-direction regression tests         | High     | Low    | 12m |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T4 | Create FEATURES.md                          | High     | Low    | 12m |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T5 | Create TODO_LIST.md                         | High     | Low    | 10m |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T6 | Create ROADMAP.md                           | Medium   | Low    | 8m  |~~ done (executed same session — see 2026-06-21_04-46 report)

### Tier P1 — High value (execute now)

| ID  | Task                                   | Impact | Effort | Est |
| --- | -------------------------------------- | ------ | ------ | --- |
~~| T7  | Move Duration() → report.go (cohesion) | Medium | Low    | 8m  |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T8  | Add Write\*String methods to Auditor   | Medium | Low    | 10m |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T9  | Add Export\* methods to WorkflowReport | Medium | Low    | 10m |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T10 | Make D2 diagram title configurable     | Medium | Low    | 8m  |~~ done (executed same session — see 2026-06-21_04-46 report)

### Tier P2 — Quality (execute now)

| ID  | Task                                      | Impact | Effort | Est |
| --- | ----------------------------------------- | ------ | ------ | --- |
~~| T11 | PeakConcurrency stress test (8+ parallel) | Medium | Low    | 10m |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T12 | CriticalPath diamond DAG test             | Medium | Low    | 10m |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T13 | Document 3 duration metrics in README     | Low    | Low    | 8m  |~~ done (executed same session — see 2026-06-21_04-46 report)

### Tier P3 — Verify & finalize

| ID  | Task                                     | Impact   | Effort | Est |
| --- | ---------------------------------------- | -------- | ------ | --- |
~~| T14 | Full test suite + race + lint + coverage | Critical | Low    | 5m  |~~ done (executed same session — see 2026-06-21_04-46 report)
~~| T15 | Update AGENTS.md with decisions made     | Medium   | Low    | 8m  |~~ done (executed same session — see 2026-06-21_04-46 report)

### Deferred to TODO_LIST.md / ROADMAP.md (high effort or needs user input)

These are documented in the living docs but NOT executed here — they are
architectural, high-effort, or require a product decision:

- flake.nix migration (deprecated justfile → nix)
- Split library into core + visualization sub-modules (dependency tax decision)
- Streaming NDJSON export
- OpenTelemetry span bridge
- encoding/json/v2 migration
- Make table columns configurable
- Diagram layout direction option (TD vs LR)
- writeToFile overwrite protection (O_EXCL)
- Fuzz tests for diagram ID sanitization
- Benchmarks for new render paths
- Version bump / v0.2.0 tag (release action — needs explicit approval)
