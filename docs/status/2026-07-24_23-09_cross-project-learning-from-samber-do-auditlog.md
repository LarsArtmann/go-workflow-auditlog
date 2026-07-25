# Status Report: Cross-Project Learning from samber-do-auditlog

**Date**: 2026-07-24 23:09  
**Session goal**: Learn from `samber-do-auditlog` and apply improvements to `go-workflow-auditlog`  
**Outcome**: 5 features ported, all tests green, but significant gaps remain

> **Update 2026-07-25:** the §E "Immediate" action items shipped in the
> follow-up session (report `2026-07-25_00-08`): export buttons added to the
> dashboard UI, CORS redesigned to secure-by-default (empty = disabled),
> `golangci-lint` clean across all 3 modules, and FEATURES/README/CHANGELOG
> updated. The §C "NOT STARTED" strategic items (JSON Schema generation,
> `MigrateReport()`, CLI tool, shared design tokens) remain unreleased — they
> are tracked as PLANNED in `FEATURES.md` and raw ideas in `ROADMAP.md`.

---

## A) FULLY DONE

### 1. Live Server: CORS Support

- **Files**: `live/server.go`, `live/server_test.go`
- **What**: `CORSAllowedOrigins` config field (default `"*"`, `"off"` disables), `corsMiddleware` wrapping all API endpoints, OPTIONS preflight handling
- **Tests**: 4 new tests (`TestServer_CORSHeaders`, `TestServer_CORSOptionsPreflight`, `TestServer_CORSDisabledWithOff`, `TestServer_CORSSpecificOrigin`)
- **Status**: Complete, passing

### 2. Live Server: Configurable Route Prefix

- **Files**: `live/server.go`, `live/dashboard.go`, `live/dashboard.js`, `live/server_test.go`
- **What**: `Prefix` config field (default `"/"`), `normalizePrefix()`, dual-route registration (avoids ServeMux 307 redirect), `window.ROUTE_PREFIX` injected into JS
- **Tests**: 2 new tests (`TestServer_PrefixRoutes`, `TestServer_PrefixTrailingSlashStripped`)
- **Status**: Complete, passing

### 3. Live Server: Export Endpoints

- **Files**: `live/server.go`, `live/server_test.go`
- **What**: `/api/export/ndjson` and `/api/export/html` endpoints with `Content-Disposition: attachment`, backed by `NDJSONWriter`/`HTMLWriter` provider types
- **Tests**: 2 new tests (`TestServer_ExportNDJSON`, `TestServer_ExportHTML`)
- **Status**: Complete, passing

### 4. CSV/TSV Export (Core Module)

- **Files**: `csv.go` (new), `csv_test.go` (new)
- **What**: `WriteCSV`, `WriteTSV`, `ExportCSV`, `ExportTSV` on `WorkflowReport`, 14 columns, nil-safe pointer formatting, semicolon-separated dependency lists, `ErrExportWriteFailed` wrapping on flush
- **Tests**: 5 new tests (write CSV, write TSV, failing writer, export CSV, export TSV)
- **Status**: Complete, passing

### 5. BENCHMARKS.md

- **File**: `BENCHMARKS.md` (new)
- **What**: All 18 benchmarks captured across core + viz, key observations documented
- **Status**: Complete (single run, not median-of-3)

### 6. Documentation Updates

- `AGENTS.md`: Added `csv.go` to source file list, updated `live/server.go` description
- `STABILITY.md`: Added new evolving API surfaces (Prefix, CORS, export endpoints, CSV/TSV)
- **Status**: Complete but partial (see B below)

---

## B) PARTIALLY DONE

### Documentation

- **FEATURES.md NOT updated** — no mention of CSV/TSV export, CORS, prefix, or export endpoints
- **README.md NOT updated** — export format table missing CSV/TSV; live dashboard section doesn't mention CORS/prefix/export
- **CHANGELOG.md NOT updated** — no entry for this session's changes
- **AGENTS.md Live module section incomplete** — `dashboard.go` description not updated to mention prefix injection, no mention of the `io` import added to server.go
- **TODO_LIST.md NOT updated** — should reflect new completed work

### Test Coverage

- **No viz CSV tests** — the viz module type-aliases WorkflowReport so CSV methods are available there too, but no viz-side test verifies CSV export through the viz package
- **No integration test** — no test wires the live server with a real workflow and verifies export endpoints serve valid data end-to-end
- **BENCHMARKS.md used count=1** — should be count=3 with benchstat for proper baseline (documented but not done)

### Dashboard JS

- **Export buttons NOT added to dashboard UI** — samber-do-auditlog has download buttons (JSON/NDJSON/HTML) in the dashboard header. The endpoints exist but the UI doesn't expose them.

---

## C) NOT STARTED

These were identified as learnings from samber-do-auditlog but NOT implemented:

1. **JSON Schema generation** (`schema.go` + `cmd/genschema/`) — samber-do-auditlog has a Draft 2020-12 JSON Schema embedded via `go:embed`, generated from Go types, with `JSONSchema()` accessor. go-workflow-auditlog has no schema.
2. **Report Migration** (`migration.go`) — samber-do-auditlog has `MigrateReport([]byte)` that upgrades older JSON reports to current schema. go-workflow-auditlog has `docs/MIGRATION.md` but no code-level migration function.
3. **CLI tool** (`cmd/auditlog/`) — samber-do-auditlog has a CLI binary (info/convert/diff/validate/schema/stats subcommands). go-workflow-auditlog has no CLI.
4. **Design tokens** (`design_tokens.go`) — samber-do-auditlog has a canonical CSS design-tokens constant shared between static HTML and live dashboard. go-workflow-auditlog duplicates CSS variables independently.
5. **`STABILITY.md` JSON Schema Versioning section** — samber-do-auditlog documents schema version independence from release tags. Missing here.
6. **Pre-commit hook for generated code** — samber-do-auditlog's pre-commit checks for templ generation drift.
7. **Coverage gate script mention in AGENTS.md** — samber-do-auditlog documents `scripts/coverage-gate.sh` extensively.
8. **go-ndjson external module** — samber-do-auditlog delegates NDJSON read/write to external `go-ndjson` module. go-workflow-auditlog has its own inline implementation.
9. **`docs/examples/` directory** — samber-do-auditlog has example docs (otel-bridge, prometheus-bridge, websocket-stream, samber-ro-adapter). No equivalent here.
10. **Property-based migration tests** — samber-do-auditlog has `migration_property_test.go`.

---

## D) TOTALLY FUCKED UP

Nothing is broken — all tests pass, vet is clean. But these are mistakes/regrets:

1. **CORS default semantics are confusing.** I defaulted empty string to `"*"` (allow all), then added `"off"` as the disable sentinel. This is backwards — the samber-do-auditlog approach is cleaner: empty = disabled (secure by default), specific string = that origin. I replicated a confusing API rather than improving on it. The `corsMiddleware` now checks `origin != "" && origin != "off"` which is a code smell — two conditions for the same concept.

2. **`renderDashboardHTML` signature change is a breaking change for tests.** I changed it from `renderDashboardHTML()` to `renderDashboardHTML(prefix string)`. This is internal, but `server_internal_test.go` constructs a bare `Server{}` struct and doesn't call `renderDashboardHTML` — it works by accident. If anyone else calls it directly, they'll break.

3. **`dashboardProvider` field is now redundant.** I changed it to `func() string { return renderDashboardHTML(cfg.Prefix) }` but `dashboardHTML` is already pre-rendered at construction. The `dashboardProvider` field is never used after construction — it's dead weight on the struct.

4. **No benchmark for CSV export.** I added CSV/TSV export but didn't add a benchmark for it, despite the BENCHMARKS.md effort. samber-do-auditlog doesn't have one either, but it would have been 5 minutes of work.

5. **The `io` and `strings` imports were added to `live/server.go`** — `strings` is used for `normalizePrefix`/`TrimSuffix`, `io` for the `NDJSONWriter`/`HTMLWriter` types. But I didn't check if `golangci-lint` would complain about import ordering or unused imports. Vet passed but lint wasn't run.

6. **The example godoc comment in `csv_test.go` is empty/useless:**

   ```go
   func ExampleWorkflowReport_WriteCSV() {
       // var report auditlog.WorkflowReport
       // report.WriteCSV(os.Stdout)
   }
   ```

   This compiles but renders as an empty example on pkg.go.dev. It should have a real runnable example.

7. **`normalizePrefix` doesn't handle double slashes.** `/workflow//` would pass through with a trailing slash stripped to `/workflow/` then... actually it trims one suffix. `/foo//` → `/foo/`. Not robust. Edge case but sloppy.

---

## E) WHAT WE SHOULD IMPROVE

### Immediate (this session's work)

1. **Add export buttons to live dashboard UI** — the endpoints exist but users have to guess URLs
2. **Fix CORS API** — make empty = disabled (secure default), remove the `"off"` sentinel hack
3. **Run `golangci-lint run`** — not run yet, may have issues
4. **Update FEATURES.md** — new features not documented
5. **Update README.md** — export table missing CSV/TSV
6. **Update CHANGELOG.md** — no entry for this session
7. **Add a real CSV benchmark** — `BenchmarkWriteCSV_LargeReport`
8. **Fix the empty Example function** — make it a real runnable example
9. **Clean up `dashboardProvider` dead field** — remove or use it
10. **Add viz-side CSV test** — verify type alias exposes the methods

### Strategic (from samber-do-auditlog patterns)

11. **Add JSON Schema generation** — `schema/report.schema.json` + `cmd/genschema` + `JSONSchema()` accessor
12. **Add `MigrateReport()` function** — programmatic schema migration, not just docs
13. **Build a CLI tool** — `cmd/auditlog` with info/convert/diff/validate subcommands
14. **Extract shared design tokens** — single CSS constant shared between viz + live
15. **Add `docs/examples/` directory** — OTel bridge, Prometheus bridge, real-world patterns
16. **Add schema-drift test** — verify Go types match embedded JSON Schema
17. **Add property-based migration test** — round-trip through migration
18. **Consider extracting NDJSON reader/writer** — into `go-ndjson` external module
19. **Add `BENCHMARKS.md` to CI** — fail on >10% regression
20. **Add coverage gate to CI** — `scripts/coverage-gate.sh` with threshold

---

## F) Up to 50 Things to Get Done Next

| #   | Task                                                                      | Priority | Effort |
| --- | ------------------------------------------------------------------------- | -------- | ------ |
| 1   | Run `golangci-lint run` on all 3 modules and fix issues                   | P0       | S      |
| 2   | Add export buttons (JSON/NDJSON/HTML) to live dashboard UI                | P0       | M      |
| 3   | Update FEATURES.md with CSV/TSV, CORS, prefix, export endpoints           | P0       | S      |
| 4   | Update README.md export format table + live dashboard features            | P0       | S      |
| 5   | Add CHANGELOG.md entry for this session                                   | P0       | S      |
| 6   | Fix CORS API: empty = disabled, remove "off" hack                         | P1       | S      |
| 7   | Fix empty Example function to be real runnable example                    | P1       | S      |
| 8   | Remove dead `dashboardProvider` field from Server struct                  | P1       | S      |
| 9   | Add `BenchmarkWriteCSV_LargeReport` benchmark                             | P1       | S      |
| 10  | Add viz-side CSV test (verify type alias exposes methods)                 | P1       | S      |
| 11  | Update AGENTS.md Live Data Flow section to mention export endpoints       | P1       | S      |
| 12  | Add integration test: real workflow → live server → export endpoint       | P1       | M      |
| 13  | Re-run benchmarks with count=3 for proper baseline                        | P1       | M      |
| 14  | Add JSON Schema generation (`schema.go` + `cmd/genschema`)                | P1       | L      |
| 15  | Add `MigrateReport()` programmatic migration                              | P1       | L      |
| 16  | Build CLI tool (`cmd/auditlog`)                                           | P2       | L      |
| 17  | Extract design tokens to shared constant                                  | P2       | S      |
| 18  | Add `docs/examples/` directory (OTel, Prometheus, WebSocket)              | P2       | M      |
| 19  | Add schema-drift test (Go types vs JSON Schema)                           | P2       | M      |
| 20  | Fix `normalizePrefix` double-slash edge case                              | P2       | S      |
| 21  | Add CORS test for non-API routes (dashboard should NOT have CORS)         | P2       | S      |
| 22  | Add OPTIONS test for export endpoints specifically                        | P2       | S      |
| 23  | Document `CORSAllowedOrigins` in README quick-start                       | P2       | S      |
| 24  | Add `ExportFilteredToFile` convenience method                             | P2       | S      |
| 25  | Consider `WriteCSVColumns` option for column selection (like table)       | P3       | M      |
| 26  | Add prefix-aware health/export URL generation helper                      | P3       | S      |
| 27  | Add live demo `--prefix` flag                                             | P3       | S      |
| 28  | Add CSV with dependencies as full StepRef (not just Name)                 | P3       | S      |
| 29  | Add TSV content-type negotiation test                                     | P3       | S      |
| 30  | Add `live.Server.URL()` helper method (returns full base URL)             | P3       | S      |
| 31  | Add websocket streaming example doc                                       | P3       | M      |
| 32  | Add Prometheus metrics bridge example                                     | P3       | M      |
| 33  | Add OTel trace bridge example                                             | P3       | M      |
| 34  | Consider rate-limiting on export endpoints                                | P3       | M      |
| 35  | Add max-step-count guard for CSV (avoid OOM on huge reports)              | P3       | S      |
| 36  | Add `Report.Stats()` method for quick summary (like CLI stats command)    | P3       | S      |
| 37  | Add `WorkflowReport.Diff` CLI documentation                               | P3       | S      |
| 38  | Add `cmd/genschema` to `flake.nix` devShell                               | P3       | S      |
| 39  | Add pre-commit hook check for schema drift                                | P3       | S      |
| 40  | Add `nix run .#auditlog` flake app for CLI                                | P3       | S      |
| 41  | Add coverage gate to CI workflow                                          | P3       | S      |
| 42  | Add `STABILITY.md` JSON Schema Versioning section                         | P3       | S      |
| 43  | Add `docs/DOMAIN_LANGUAGE.md` update for new terms (CORS, prefix, export) | P3       | S      |
| 44  | Add `CONTRIBUTING.md` mention of CSV export                               | P3       | S      |
| 45  | Consider CSV escaping tests for step names with commas/quotes             | P3       | S      |
| 46  | Add fuzz test for CSV injection (formula injection via step names)        | P3       | M      |
| 47  | Add `live.Config.Addr()` validation (reject invalid addresses early)      | P3       | S      |
| 48  | Add graceful SSE disconnect on server shutdown                            | P3       | M      |
| 49  | Add request logging middleware for live server                            | P3       | S      |
| 50  | Add `docs/status/INDEX.md` entry for this report                          | P3       | S      |

---

## G) Questions

1. **CORS default**: Should I change the default to secure-by-default (empty = no CORS headers, explicit `"*"` needed to allow all)? This would be a behavior change for anyone relying on the default, but we're ALPHA so it's acceptable. Or keep the convenient `"*"` default matching samber-do-auditlog?

2. **CLI tool scope**: The samber-do-auditlog CLI has 6 subcommands (info/convert/diff/validate/schema/stats). Should I build the full CLI, or just the essentials (info + convert + validate) first? The CLI is a significant effort and the library is ALPHA — is it worth investing in before the API stabilizes?

3. **JSON Schema**: Should the schema cover only `WorkflowReport` (the top-level export type), or also `Event`, `StepInfo`, and the streaming NDJSON format? samber-do-auditlog only schemas the report — should I match that scope, or go broader?
