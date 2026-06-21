# Status Report — go-workflow-auditlog — 2026-06-21

**Version**: 0.1.1 (unreleased changes pending) · **Go**: 1.26.3 · **Module**: `github.com/larsartmann/go-workflow-auditlog`

---

## Build & Quality Gates

| Gate        | Status  | Detail                          |
| ----------- | ------- | ------------------------------- |
| Build       | ✅ PASS  | `go build ./...` exits 0        |
| Tests       | ✅ PASS  | 164 pass, 0 fail (`-race`)      |
| Coverage    | ✅ 93.1% | auditlog package                |
| Lint        | ✅ 0 issues | `golangci-lint run ./...`    |
| Vet         | ✅ PASS  | `go vet ./...` exits 0          |
| LSP Warning | ⚠️ PERSISTENT | `d2.go:9` typecheck warning (see Fucked Up #2) |

**Codebase**: 38 source files, 14 test files, ~7,032 LOC total.

---

## A) FULLY DONE ✅

### Core Audit Pipeline
- `Auditor` with `Attach(w)` → `w.Do(ctx)` → `Snapshot(w)` lifecycle
- Per-attempt event capture: `attempt_start` / `attempt_end` with timestamps, errors, durations
- Full step DAG capture: dependencies, dependents, retry/timeout config, step types
- Sub-workflow traversal via `flow.Traverse` (captures inner steps that bypass callbacks)
- Skipped & canceled detection (reads post-execution state for steps that bypass Before/After)
- `StepInfo.Error` reflects FINAL outcome only (regression-tested)

### Report & Query API
- `WorkflowReport` with denormalized aggregates (counts, durations, peak concurrency, critical path)
- `Validate()` — checks count consistency + status drift via sentinel errors
- `Filtered(opts...)` — filter by step name, status, event type, time range
- `Diff(other)` — compare two runs (added/removed/changed steps + wall-clock duration delta)
- `Summary()` — one-line human-readable summary
- `Duration()` — wall-clock duration (earliest → latest event)
- `WallClockDurationMs` field — actual elapsed time (vs summed step durations)
- `ReportIndex` — O(1) lookup maps for repeated queries
- `ReplayEvents()` — reconstruct report from flat NDJSON event stream
- `LoadReport()` / `LoadReportFromReader()` / `LoadReportFromBytes()`
- `ReadEvents()` — NDJSON reader (inverse of WriteNDJSON)

### Export Formats (all working, all tested)
- **JSON** report (`WriteJSON` / `ExportToFile`)
- **NDJSON** event stream (`WriteNDJSON` / `ExportEventsToNDJSON`)
- **Mermaid** flowchart (`WriteMermaid` / `ExportMermaid`)
- **PlantUML** component diagram (`WritePlantUML` / `ExportPlantUML`)
- **Graphviz DOT** digraph (`WriteGraphviz` / `ExportGraphviz`)
- **D2** diagram (`WriteD2` / `ExportD2`)
- **Table** — 16 formats via go-output (`WriteTable` / `ExportTable`): table, json, csv, tsv, markdown, xml, d2, yaml, html, tree, mermaid, dot, jsonl, asciidoc, toml, plantuml
- **ASCII Tree** (`WriteTree` / `ExportTree`)
- **HTML Tree** (`WriteHTMLTree` / `ExportHTMLTree`)

### Architecture Quality
- **Single source of truth for colors**: `StepStatus.Color()` in `types.go` — all diagram renderers delegate to it
- **Single DAG builder**: `buildGraph()` feeds Mermaid, Graphviz, PlantUML, and D2 (via thin converters)
- **Single label function**: `stepLabel()` used by all renderers (includes status text + retry indicator)
- **Single report construction path**: `buildReportFromCore()` used by `BuildReport`, `Filtered`, and `ReplayEvents`
- **Concurrency model**: single `sync.RWMutex`, `atomic.Int64` for sequence, `OnEvent` fires outside lock

### Documentation
- `AGENTS.md` — comprehensive, updated with all new formats and correct coverage (93.1%)
- `README.md` — updated with all export methods in API reference tables + usage examples
- `CHANGELOG.md` — exists but [Unreleased] section is empty (see Not Started)
- `example/main.go` — demos all export formats via `--export` flag

---

## B) PARTIALLY DONE ⚠️

### 1. API Symmetry Between `Auditor` and `WorkflowReport`

`Write*` methods exist on **both** `Auditor` and `WorkflowReport`. But:

| Method type | On `Auditor`? | On `WorkflowReport`? |
|---|---|---|
| `Write*` | ✅ All | ✅ All |
| `Write*String` | ❌ NONE | ✅ All 7 |
| `Export*` | ✅ All | ❌ NONE |

A user holding a `WorkflowReport` (e.g., from `ReplayEvents`) **cannot** export to file — `Export*` only exists on `Auditor`. A user holding an `*Auditor` must call `a.Report().WriteMermaidString()` instead of `a.WriteMermaidString()`.

### 2. JSON/NDJSON Naming Breaks the Pattern

Every diagram/table/tree format follows `Write<Format>` / `Export<Format>`. But JSON/NDJSON diverge:

| Expected | Actual on `Auditor` | Actual on `WorkflowReport` |
|---|---|---|
| `WriteJSON` | `WriteReportJSON` | `WriteJSON` |
| `ExportJSON` | `ExportToFile` | — (no Export) |
| `WriteNDJSON` | `WriteEventsNDJSON` | `WriteNDJSON` |
| `ExportNDJSON` | `ExportEventsToNDJSON` | — (no Export) |

### 3. Diagram Titles Inconsistent

`WriteD2` sets a hardcoded title `"Workflow DAG"` (`d2.go:50`). Mermaid, Graphviz, and PlantUML set **no title** at all. The title is not configurable by the caller.

### 4. CHANGELOG [Unreleased] is Empty

Despite 7 commits since v0.1.1 adding D2/table/tree exports, color consolidation, label unification, and buildD2Graph elimination, the `[Unreleased]` section in `CHANGELOG.md` has empty Added/Changed/Fixed subsections.

---

## C) NOT STARTED ❌

| Item | Notes |
|---|---|
| `FEATURES.md` | Does not exist. No honest feature inventory by status. |
| `TODO_LIST.md` | Does not exist. No actionable short/mid-term task list. |
| `ROADMAP.md` | Does not exist. No long-term direction. |
| `docs/DOMAIN_LANGUAGE.md` | Does not exist. No DDD glossary. |
| `flake.nix` | Does not exist. Project uses `.goreleaser.yml` + `justfile` (deprecated per AGENTS.md). BuildFlow pre-commit hook runs checks but there's no `nix build` / `nix flake check` path. |
| Table column configuration | `buildTableData()` hardcodes columns: Step, Status, Duration, Attempts, Error. No way for users to customize which columns appear or add custom columns. |
| Configurable diagram direction | No way to set Mermaid/D2/Graphviz layout direction (TD vs LR). |
| `writeToFile` overwrite protection | `os.Create` truncates unconditionally — no `O_EXCL` or "file exists" check. |

---

## D) TOTALLY FUCKED UP 🚨

### 1. Edge Direction is OPPOSITE Between Diagrams and Tree

This is the most significant UX bug. The two visual representations of the same DAG **disagree on arrow direction**:

**Diagrams** (`diagram.go:64-68`): edges point **step → dependency** (backward against execution flow):
```
If step A depends on B, the arrow is A → B (points backward)
```

**Tree** (`tree.go:29-36, 56`): roots are steps with no dependencies, children are **dependents** (forward execution flow):
```
B (root) → A (child)  — reads top-down in execution order
```

So a workflow `fetch → transform → save` renders in the tree as `fetch` at top flowing down to `save`, but in Mermaid/Graphviz/D2/PlantUML as `save → transform → fetch` (arrows pointing backward). **No test asserts edge direction** — tests only check that *some* edge exists.

### 2. go.mod Version Skew Causes Persistent LSP Warning

```
github.com/larsartmann/go-output v0.17.0        ← root
github.com/larsartmann/go-output/d2 v0.13.0      ← all sub-modules
github.com/larsartmann/go-output/graph v0.13.0
github.com/larsartmann/go-output/tree v0.13.0
... (all at v0.13.0)
```

This produces a permanent typecheck warning in the IDE:
```
d2.go:9:2: no required module provides package github.com/larsartmann/go-output/d2
```

The project **builds and tests fine** (the MVS resolution works), but `gopls`/the LSP cannot resolve the `d2` package. This is likely because the root module API (v0.17.0) has drifted from what the v0.13.0 sub-modules expect. The warning is cosmetic but erodes confidence and may indicate a real compatibility risk.

---

## E) WHAT WE SHOULD IMPROVE

### Type Model & Architecture
1. **Eliminate Auditor→Report delegation boilerplate.** Every `Auditor.Write*` is `return a.Report().Write*()` — 15+ one-liner methods. Consider embedding `WorkflowReport` or providing a generic `Export(format, w)` dispatcher.
2. **Add `StepStatus.Icon()` and `Color()` to a unified `StatusVisual` struct** — currently label/icon/color are separate map lookups in `stepStatusMeta`. A single struct return would be cleaner.
3. **Use `go-error-family` for structured errors** — the go-structure-linter flags this. Current sentinel errors (`ErrEventCountMismatch`, etc.) are plain `errors.New` variables.

### Library Usage
4. **Pin go-output sub-modules to compatible versions** — either downgrade root to v0.13.0 or upgrade sub-modules. Eliminates the LSP warning.
5. **Migrate `justfile` to `flake.nix`** — AGENTS.md says justfile is deprecated; BuildFlow already runs the checks, but `nix build` / `nix flake check` should be the canonical path.

### Testing & Quality
6. **Add edge-direction assertions to diagram tests** — tests currently only check "an edge exists", not which way it points. This allowed the direction bug to persist.
7. **Add `FEATURES.md` and `TODO_LIST.md`** — no honest feature inventory or task tracking exists.
8. **Fill in CHANGELOG [Unreleased]** — 7 commits of significant work are undocumented.

---

## F) Top 25 Things to Get Done Next

Ranked by impact/effort ratio (highest first):

| # | Task | Impact | Effort | Category |
|---|------|--------|--------|----------|
| 1 | **Fix edge direction in diagrams** — reverse to match execution flow (dependency → step) so all visual representations agree | HIGH | LOW | Bug fix |
| 2 | **Add edge-direction test assertions** — prevent regression of #1 | HIGH | LOW | Testing |
| 3 | **Fill CHANGELOG [Unreleased]** — document D2/table/tree additions, color consolidation, refactors | HIGH | LOW | Docs |
| 4 | **Make D2 title configurable or remove it** — either let caller set title or drop the hardcoded `"Workflow DAG"` for consistency with other diagrams | MED | LOW | Polish |
| 5 | **Add `Export*` methods to `WorkflowReport`** — so reports from `ReplayEvents` can write to files without an `Auditor` | HIGH | LOW | API |
| 6 | **Rename JSON/NDJSON methods for consistency** — `WriteReportJSON` → `WriteJSON`, `ExportToFile` → `ExportJSON`, `ExportEventsToNDJSON` → `ExportNDJSON` (keep deprecated aliases) | MED | LOW | API |
| 7 | **Add `Write*String` methods to `Auditor`** — mirror the 7 `WorkflowReport.Write*String` methods | MED | LOW | API |
| 8 | **Resolve go-output version skew** — pin all sub-modules to compatible version, eliminate LSP warning | HIGH | MED | Infra |
| 9 | **Create `FEATURES.md`** — honest feature inventory by status (DONE/PARTIALLY/PLANNED) | MED | LOW | Docs |
| 10 | **Create `TODO_LIST.md`** — actionable short/mid-term tasks from this report | MED | LOW | Docs |
| 11 | **Add diagram direction option** — let caller choose TD vs LR for Mermaid/D2/Graphviz | LOW | LOW | Feature |
| 12 | **Make table columns configurable** — `WriteTable` currently hardcodes 5 columns | MED | MED | Feature |
| 13 | **Add `WriteHTMLTable` dedicated method** — currently accessible only via `WriteTable(w, FormatHTML, opts)` but not as a dedicated method | LOW | LOW | API |
| 14 | **Split `tree.go` into `tree.go` + `htmltree.go`** — HTMLTree breaks the one-format-per-file convention | LOW | LOW | Structure |
| 15 | **Add `flake.nix`** — migrate from deprecated justfile to nix flake build automation | MED | MED | Infra |
| 16 | **Add `docs/DOMAIN_LANGUAGE.md`** — DDD glossary for audit/log/workflow vocabulary | LOW | LOW | Docs |
| 17 | **Consider `go-error-family` adoption** — structured, classified errors per linter recommendation | LOW | MED | Arch |
| 18 | **Add `writeToFile` overwrite protection** — `O_EXCL` flag or "file exists" error | LOW | LOW | Safety |
| 19 | **Surface name collisions in diagrams** — when two steps share `String()`, the `seen` map silently merges them; should warn | LOW | MED | UX |
| 20 | **Add retry/timeout columns to table export** — `HasRetry` and `HasTimeout` are in `StepInfo` but not in the table | LOW | LOW | Feature |
| 21 | **Add `StepInfo.Type()` method** — expose `StepType` via a method for consistency with `Status.Label()` / `Icon()` / `Color()` | LOW | LOW | API |
| 22 | **Consider HTML report generation** — a self-contained HTML dashboard combining table + diagram + tree | MED | HIGH | Feature |
| 23 | **Add benchmark for large workflows** — 100+ steps, measure report build + export latency | LOW | MED | Perf |
| 24 | **Add ` ROADMAP.md`** — long-term direction (HTML reports, CLI tool, OpenTelemetry integration) | LOW | LOW | Docs |
| 25 | **Consider streaming export** — for very large event streams, write events as they're captured rather than buffering all in memory | LOW | HIGH | Arch |

---

## G) Top Question I Cannot Answer Myself

**Should the edge direction in diagrams be reversed to match execution flow (like the tree), or should the tree be reversed to match dependency direction (like the diagrams)?**

Currently:
- **Diagrams** (Mermaid/Graphviz/D2/PlantUML): `step → dependency` (A depends on B → arrow A→B)
- **Tree**: `root → dependent` (B is root, A is child → reads top-down B→A)

These are **opposite**. I don't know which direction is the "correct" one for the user's mental model:
- **Option A**: Reverse diagram edges to `dependency → step` (B→A), matching the tree. Arrows follow execution flow. This is the more common convention in pipeline/DAG visualizations (e.g., GitHub Actions, Airflow).
- **Option B**: Reverse the tree to show dependencies as children (A is root, B is child). Matches the diagram's current direction. Less intuitive for execution-order reading.

I lean toward **Option A** (make diagrams match the tree's execution-flow direction), but this changes the visual output of every existing diagram and could surprise users who've already integrated the current output.
