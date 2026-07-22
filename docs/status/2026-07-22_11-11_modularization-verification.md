# Status Report — go-workflow-auditlog Modularization

**Date:** 2026-07-22 11:12 CEST  
**Branch:** `modularize/core-viz-split`  
**Snapshot:** Manual verification run after core-viz module split.  
**Prepared by:** Crush (automated assistant)

---

## Executive Summary

The core-vs-visualization module split is **complete and verified**. The branch is 21 commits ahead of origin, the working tree is clean, and all manual verification gates pass. Two near-misses were caught during verification: a deprecated `archives.builds` field in `.goreleaser.yml` and a core coverage drop to 88.1% after moving viz tests out. Both are fixed. The remaining work is release logistics and follow-up polish.

| Metric                   | Value    | Target | Status |
| ------------------------ | -------- | ------ | ------ |
| Core tests               | pass     | pass   | done   |
| Viz tests (workspace)    | pass     | pass   | done   |
| Viz tests (`GOWORK=off`) | pass     | pass   | done   |
| Core lint                | 0 issues | 0      | done   |
| Viz lint                 | 0 issues | 0      | done   |
| Core coverage            | 94.1%    | ≥92%   | done   |
| Viz coverage             | 92.7%    | ≥92%   | done   |
| `goreleaser check`       | valid    | valid  | done   |
| `go work sync`           | clean    | clean  | done   |
| `git status`             | clean    | clean  | done   |

---

## a) FULLY DONE

1. **Module split implemented.** Core module (`github.com/larsartmann/go-workflow-auditlog`) owns event model, recorder, report, JSON/NDJSON, replay, diff, filter, index, and error classification. Viz module (`.../viz`) owns diagrams, tables, trees, and HTML dashboard.
2. **Dependency isolation.** Core no longer depends on `go-output`. Viz depends on core + `go-output`.
3. **Shared test fixtures.** `testhelpers` package extracted inside the core module so both core and viz tests can share fixtures without circular module dependencies.
4. **Viz API converted.** All viz production files moved to `viz/` and converted from `WorkflowReport` methods to standalone package-level functions (`viz.WriteMermaid(report, ...)` etc.).
5. **Re-exports fixed.** `viz/viz.go` wraps `auditlog.WriteToFile`, `AllStepStatuses`, and `AllEventTypes` as exported functions to avoid global-variable lint errors and unexported core references.
6. **Example moved.** `example/` moved to `viz/example/` and updated to use `viz` package functions.
7. **Workspace wired.** `go.work` and `go.work.sum` created with `.` and `./viz`.
8. **CI updated.** `.github/workflows/ci.yml` runs per-module vet/build/test-lint, standalone `GOWORK=off` viz verification, coverage gate, vulncheck, and go-mod-tidy drift check.
9. **Goreleaser updated.** `main` changed to `./viz/example`, hooks added for viz module, and `archives.builds` deprecated field replaced with `ids` so `goreleaser check` passes.
10. **Docs updated.** `AGENTS.md`, `README.md`, and five website docs pages updated for the new modular API.
11. **Core coverage restored.** Added `coverage_report_test.go` to cover report query methods, enumerators, export paths, and summary branches after viz tests moved out.
12. **Final verification completed.**

- `go mod tidy` in both modules.
- `GOEXPERIMENT=jsonv2 go test ./...` passes in core and viz.
- `GOWORK=off GOEXPERIMENT=jsonv2 go test ./...` passes in viz.
- `golangci-lint run ./...` 0 issues in core and viz.
- Race + coverage: core 94.1%, viz 92.7%.
- `go work sync` clean.
- `goreleaser check` valid.

---

## b) PARTIALLY DONE

1. **Coverage is adequate but not generous.** Core is at 94.1% and viz at 92.7%, both above the 92% gate, but there is little headroom. Some unexported helpers (`sortByName`, `fromFlowStatus` fallback branch) are only indirectly covered.
2. **Website docs updated mechanically.** The content was migrated to the new `viz` import paths, but a human readability pass for examples and flow could still help.
3. **`testhelpers` is public but not documented.** It is now part of the public API surface, yet it lacks package-level examples and a clear contract for external consumers.
4. **Local verification is manual.** The verification commands are documented in `AGENTS.md`, but there is no single `nix run .#check` or `just check` command that runs the full matrix locally.
5. **Gopls warnings remain.** ~46 non-blocking diagnostics (benchmark `b.N` modernization, jsonv2 stdversion warnings) are still visible in the project.

---

## c) NOT STARTED

1. Push `modularize/core-viz-split` to origin.
2. Create a PR and let CI run the full matrix.
3. Merge to `master`/`main`.
4. Update `CHANGELOG.md` for the modularization release.
5. Decide on and apply the next alpha version tag.
6. Publish GitHub release via goreleaser.
7. Verify release artifacts (demo binaries) on Linux and Darwin.
8. Write a consumer migration guide (old method-based API → new `viz` function API).
9. Add package-level examples for `viz` functions and core-only usage.
10. Generate the HTML dashboard variant of this status report (skill default).
11. Add performance regression tests for the modularized build.

---

## d) TOTALLY FUCKED UP!

1. **Deprecated goreleaser config nearly shipped.** `archives.builds` was still present when `goreleaser check` was run; it would have failed in CI or local release workflows using the current GoReleaser version. Caught and fixed during verification.
2. **Core coverage dropped below the gate after modularization.** Moving viz tests out reduced core coverage from ~95.6% to 88.1%, which would have failed the CI coverage gate. Caught during manual verification and fixed by adding `coverage_report_test.go`.
3. **Session handoff/context confusion.** The working tree was already clean and committed when this session began, but the handoff context described remaining work as incomplete. This led to redundant edits and temporary confusion about what still needed to be done. The final verification was still valuable, but the initial state was ambiguous.
4. **Gopls noise.** ~46 warnings persist in the project. They are not blocking but make the codebase look less polished and can hide real issues.

---

## e) WHAT WE SHOULD IMPROVE!

1. **Catch goreleaser deprecations in CI.** Add a `goreleaser check` job to the CI workflow so deprecations and config errors are caught before manual release time.
2. **Add a local all-checks command.** Create a `nix run .#check` or shell script that runs mod tidy, vet, tests with `GOWORK=off`, lint, race coverage, and goreleaser check in one command.
3. **Document the coverage gate per module.** Explain why core is 92% and how viz is measured so future contributors understand the threshold.
4. **Add internal tests for unexported helpers.** `sortByName`, `fromFlowStatus` fallback, and similar small helpers should be covered directly to push core coverage toward 95%+ without relying on synthetic reports.
5. **Reduce gopls warnings.** Modernize benchmarks to `b.Loop()` and address or suppress jsonv2 stdversion diagnostics so the project looks clean in editors.
6. **CHANGELOG for breaking change.** The modularization is a breaking API change for viz consumers; it needs a clear CHANGELOG entry and migration notes.
7. **Clarify `testhelpers` public contract.** Add examples and a note about whether consumers should import it or it is meant for internal use only.
8. **Consider `testhelpers` placement.** If it is purely for tests, evaluate moving it to `internal/testhelpers` to reduce the public API surface.
9. **Add pre-push checks.** A pre-push hook that runs `golangci-lint` and `go test` would prevent lint and coverage regressions from reaching CI.
10. **Improve error-path coverage.** Several export/write error paths are present but coverage is thin; targeted tests would improve confidence.

---

## f) Top 50 Things to Get Done Next

### Release & Logistics

1. Push `modularize/core-viz-split` to origin.
2. Open a PR against `master`/`main`.
3. Let CI run the full matrix and confirm green.
4. Merge the PR.
5. Update `CHANGELOG.md` with modularization notes and breaking changes.
6. Decide the next alpha version tag (e.g., v0.3.0).
7. Apply the version tag.
8. Run `goreleaser release --snapshot --clean` locally as a dry run.
9. Publish the GitHub release.
10. Verify release artifacts (demo binaries) on Linux and Darwin.

### Documentation & Consumer Experience

11. Add a consumer migration guide (`docs/guides/migration.md` or README section).
12. Add package-level examples for `viz` functions.
13. Add package-level examples for core-only usage.
14. Review website docs for tone and example accuracy after the API change.
15. Document the public `testhelpers` API and intended use.
16. Add an API stability policy note to README.
17. Add issue templates for bug reports and feature requests.
18. Add a PR template.
19. Add `CONTRIBUTING.md`.
20. Add a coverage badge and CI status badge to README.

### Testing & Coverage

21. Add internal tests for `sortByName` and `fromFlowStatus` fallback.
22. Add tests for `WriteToFile` atomicity failure paths.
23. Add tests for `CheckNoClobber` and `ErrFileExists`.
24. Add test for concurrent `Snapshot`/`Attach` usage.
25. Add test for sub-workflow visualization edge cases.
26. Add test for diamond DAG critical path.
27. Add test for peak concurrency with retries.
28. Add test for empty report exports.
29. Add test for error classification of all new sentinel paths.
30. Add test for HTML dashboard from replayed report.
31. Add test for table export from replayed report.
32. Add fuzz tests for `ReplayEvents`.
33. Add property tests for `ReportIndex`.
34. Add performance regression tests for the modularized build.
35. Add integration test importing only the core module.
36. Add integration test importing the viz module with `GOWORK=off`.

### Tooling & CI

37. Add a CI `goreleaser check` job.
38. Add a local `nix run .#check` or equivalent all-checks command.
39. Add `govulncheck` to local checks.
40. Add a script to detect `go.mod`/`go.sum` drift across modules.
41. Verify actionlint is actually running in CI (config exists, confirm execution).
42. Verify release hooks handle `go.work` correctly in CI.
43. Add CI check that fails on gopls warnings (optional).
44. Set up dependabot for Go module updates.
45. Automate website deployment on release tag.

### Polish & Refactoring

46. Modernize benchmarks to use `b.Loop()`.
47. Resolve or suppress jsonv2 stdversion warnings.
48. Review public API surface for unnecessary exports.
49. Evaluate moving `testhelpers` to `internal/testhelpers`.
50. Add a top-level `docs/ARCHITECTURE.md` explaining the core-vs-viz split.

---

## g) Questions I Cannot Answer Without You

1. **What alpha version tag should I use for the release after merging?** (e.g., `v0.3.0`, `v0.2.1-alpha`, another scheme?)
2. **Should I push the branch and create the PR now, or do you want to review the final diff first?**
3. **Should I also generate the HTML dashboard version of this status report per the `status-report` skill, or is this Markdown file sufficient?**

---

_Report written at 2026-07-22 11:12 CEST. Waiting for further instructions._
