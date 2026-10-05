# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks, verified against the actual code on 2026-10-05.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md) — never retained here.

---

## Release close-out (v0.11.0 hygiene)

- [ ] **STABILITY.md: classify the v0.9.0 + v0.11.0 public API** — `MigrateReport`, `JSONSchema()`, `RedriveReportStatuses`, the migration sentinels, the CLI subcommands, `MarkCached`, `ErrMarkCachedNoStepContext`, `WithCachedSteps`/`WithUncachedSteps`, the `Diff()` cached fields, and `ColumnCached` appear in no stability table. Default everything new to Evolving.
      _Source: `docs/status/2026-09-11_15-36` §f1, `docs/status/2026-08-12_09-50` §f16_
- [ ] **README "Example Output" refresh** — the block still shows the pre-v0.11.0 six-step demo run; the demo now has a `detect` step and prints a `Cached: N (results reused, not re-verified)` summary line. pkg.go.dev renders the stale block for v0.11.0.
      _Source: `docs/status/2026-09-11_15-36` §f2_
- [ ] **Fix the release skill's workspace-mode claim** — `skills/release/SKILL.md` Phase 4 still implies the workspace resolves sibling modules locally after a version bump. Since Go 1.26.7, a workspace member's required sibling version resolves from the module proxy even with `use` directives, so bumped `go.mod` files break ALL local builds until the tags are pushed. Verify pre-bump; bump → tag → push promptly.
      _Source: `docs/status/2026-09-11_15-36` §f3/§d4_
- [ ] **Untrack stray generated dashboards at repo root** — `workflow-audit-log-20260911-080328-e614c0a7.html` and `dashboard.html` are tracked in git although `.gitignore` covers the pattern. `git rm --cached` both (keep on disk if wanted).
      _Source: `docs/status/2026-09-11_10-38` §d2/§f6_

## Infrastructure

- [ ] **`nix build .#auditlog` under Go 1.27** — `packages.default` builds via `buildGo126Module` while all three `go.mod` files now require `go 1.27`. Verify the package still builds (or switch to a 1.27-capable builder); the empty-marker predecessor was replaced in f92c168, don't let the real package rot the same way.
- [ ] **CI: add `go mod verify`** — the mod-tidy job checks drift, but nothing verifies module-cache checksums against `go.sum`.
      _Source: `docs/status/2026-06-18_19-12` §f13_
- [ ] **erraudit CI gate activation** — the `Error audit` job green-skips until a `PRIVATE_REPO_PAT` secret (fine-grained PAT, Contents:read on the private `larsartmann/erraudit` repo) is added — or erraudit goes public. Needs repo-owner action; then the gate runs for real.
      _Source: `docs/status/2026-09-11_09-55` §e3/§f3_
- [ ] **`scripts/test-count.sh`** — print per-module test-function counts (`rg -c '^func (Test|Example|Benchmark|Fuzz)' -g '*_test.go'`) so prose numbers in AGENTS.md stop rotting (second incident: "481" claimed vs 570 actual).
      _Source: `docs/status/2026-09-11_15-36` §f14, `docs/status/2026-09-11_13-49` §e3_

## Quality

- [ ] **Dashboard CSS percentage-height bug class** — audit `#timeline-container` and `.graph-minimap` for the same `min-height` + percentage-sized-child bug that was fixed for `#graph-container` (b67e91d); add a CSS regression test asserting a definite graph-container height; document the CSS layering rule (viz CSS = base layer, live CSS = overlay, load order is viz-then-live) in AGENTS.md Gotchas.
      _Source: `docs/status/2026-09-11_10-38` §f3/§f7/§f12/§f15_
- [ ] **Timeline/Gantt cached bars + `BenchmarkMarkCached`** — striped fill using the `--cache` token in both dashboards; plus a benchmark proving the mutex-per-hit overhead of `MarkCached` is negligible.
      _Source: `docs/status/2026-09-11_13-49` §f23/§f27_
- [ ] **Live-module error-path sentinels** — give the hub drain error a sentinel (`live/hub.go`), add a Corruption sentinel for "replayed report failed validation" and a Rejection sentinel for the bare migration unmarshal path; store the numeric sequence in the event ring buffer so the parse-skip branch and its nolint disappear.
      _Source: `docs/status/2026-09-11_07-55` §f8-10/§f17_

## Website

- [ ] **Missing guide pages** — error classification, retry & timeout tracking, concurrency model, replay & load (existing guides cover event-stream, filtering/diffing, export formats, and the HTML dashboard only). While there: refresh the features page and dashboard screenshot to show the `⚡ cached` badges.
      _Source: `docs/status/2026-07-13_21-17` §f12-15, `docs/status/2026-09-11_13-49` §f26_
