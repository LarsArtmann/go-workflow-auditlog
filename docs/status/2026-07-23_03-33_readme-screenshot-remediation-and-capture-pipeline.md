# Status Report: README Screenshot Remediation & Reproducible Capture Pipeline

**Date**: 2026-07-23 03:33
**Session scope**: Remediate previous session's mistakes (clobbered files, unused asset, fragile screenshots), build robust reproducible capture pipeline, re-capture all dashboard tabs with sufficient JS rendering time
**Branch**: master (commit `36136ef`)
**Previous review**: `docs/status/2026-07-22_19-03_readme-screenshot-enhancement-review.md`

---

## a) FULLY DONE

1. **Restored clobbered reference files** — Previous session accidentally deleted `dashboard.html`, `steps.csv`, `steps-compact.md` (tracked in git) via `trash`, then regenerated them with non-deterministic RunID/timestamps. Verified all 8 tracked generated files (`dashboard.html`, `steps.csv`, `steps-compact.md`, `dag.d2`, `dag.dot`, `dag.mmd`, `dag.puml`, `tree.txt`) match committed state at HEAD — 0 lines of diff.

2. **Removed unused `example-hero.png`** — 204KB PNG tracked in git but never referenced in README. `git rm`'d and confirmed gone from `git ls-files`.

3. **Built reproducible screenshot capture script** (`scripts/capture-screenshots.sh`) — 90-line bash script that:
   - Builds the example binary to a temp dir (prevents clobbering tracked files)
   - Runs `--export` from that temp dir
   - Injects `switchTab()` call directly via `sed` before `</body>` (synchronous, guaranteed available)
   - Uses `timeout 30` wrapper (chromium hangs on GCM background tasks after writing PNG)
   - Uses `--virtual-time-budget=10000` (10 seconds for full JS execution)
   - Uses `--disable-background-networking`, `--disable-extensions`, `--disable-sync`, `--no-first-run`
   - Resizes to 1200px wide via ImageMagick with `-strip -quality 85`
   - Verifies uniqueness via `md5sum`

4. **Re-captured all 4 dashboard tab screenshots** at 2x retina (1280x900 viewport, scaled to 1200x844):
   - `example-graph.png` (164KB) — DAG Graph tab with rendered SVG
   - `example-steps.png` (204KB) — Steps table tab
   - `example-timeline.png` (162KB) — Timeline bar chart tab
   - `example-tree.png` (178KB) — DAG Tree tab
   - All 4 have unique MD5 checksums (verified)

5. **Updated README.md with proper image attributes**:
   - Hero image (line 31): Changed from `example-steps.png` to `example-graph.png` (more visually compelling), added `width="600" height="422"` for CLS prevention
   - Gallery (lines 549-566): Added `loading="lazy"`, `width="400" height="281"`, descriptive WCAG-compliant alt text on all 4 images

6. **All changes committed** — Pre-commit hook swept everything into commit `36136ef` alongside concurrent agent work.

7. **Tests pass, vet clean** — `GOEXPERIMENT=jsonv2 go test ./...` and `go vet ./...` both pass.

---

## b) PARTIALLY DONE

1. **TOC entry for screenshots** — README has a Table of Contents (line 48) but no "Screenshots" or "Visual Output" entry was added. The screenshots are embedded inline in the HTML Dashboard section, so a dedicated TOC entry is debatable.

2. **Visual verification of screenshots** — File sizes and MD5 uniqueness confirm the 4 screenshots are different, but the model cannot view images to verify the DAG Graph SVG is fully rendered (nodes, edges, labels visible). The `--virtual-time-budget=10000` should be more than sufficient given the graph renders synchronously via pure math (no DOM layout queries), but this remains unconfirmed by human eyes.

---

## c) NOT STARTED

1. **Events tab screenshot** — The 5th dashboard tab (Events stream) was not captured. Only 4 of 5 tabs are in the gallery.
2. **Failed-step scenario screenshot** — All screenshots show all-green (succeeded) steps. A scenario with red/failed statuses would be more compelling for README visitors.
3. **CONTRIBUTING.md documentation** — The capture script exists at `scripts/capture-screenshots.sh` but is not documented in CONTRIBUTING.md or AGENTS.md.
4. **CI integration** — No automation to regenerate screenshots when dashboard CSS/JS changes (screenshots will drift).
5. **Website integration** — Screenshots only in README; the Astro website (`website/src/`) could use them in HeroSection or the HTML dashboard guide.

---

## d) TOTALLY FUCKED UP

1. **Concurrent agent collision** — During this session, another agent (Crush:MiniMax-M3) was modifying the same repo concurrently. It modified `viz/daghtml_adapter.go`, `viz/dashboard.js`, `AGENTS.md`, `CHANGELOG.md`, `FEATURES.md`, `TODO_LIST.md`, and added two status docs. The pre-commit hook merged all changes (mine + theirs) into a single commit `36136ef` with the other agent's attribution. My README image edits were stripped twice by the concurrent agent's writes and had to be re-added. This was not something I could control, but I should have been more defensive about re-reading files before each edit.

2. **First `--export` run clobbered tracked files again** — At the start of this session, I ran `GOEXPERIMENT=jsonv2 go run ./viz/example --export` from the project root, which clobbered all 8 tracked generated files before I realized the example binary writes to CWD with no output-dir flag. I fixed this by restoring from `ff9204e` and updating the capture script to build+run from a temp dir, but I repeated the exact same mistake from the previous session before catching it.

3. **Chromium process management** — The first test capture (steps tab) succeeded but chromium hung indefinitely on GCM registration background tasks. I had to kill it manually and add `timeout 30` to the script. Should have used `timeout` from the start.

---

## e) WHAT WE SHOULD IMPROVE

1. **Add `--output-dir` flag to the example binary** — The root cause of the clobbering issue is that `viz/example/main.go` hardcodes output paths to CWD. Adding an `--output-dir` flag would eliminate this entire class of mistake permanently.

2. **Check `git ls-files` before deleting ANY file** — This was flagged in the previous review and I still ran `--export` from project root without checking. The lesson needs to be encoded as a hard rule.

3. **Use `timeout` for all headless browser commands** — Chromium's background networking makes it hang after completing the screenshot. `timeout 30` is mandatory.

4. **File locking or session isolation** — The concurrent agent collision caused real damage (README edits stripped twice). If multiple agents work on the same repo, there should be a coordination mechanism.

5. **WebP format instead of PNG** — Screenshots are 160-204KB each as PNG. WebP would be ~40-60% smaller for the same quality. GitHub renders WebP in README.

6. **Animated GIF or video** — A static screenshot of an interactive dashboard undersells the pan/zoom/click-highlight features. A 5-second screen recording would be far more compelling.

---

## f) Up to 50 Things to Do Next

### Immediate polish

1. Add "Screenshots" or "Visual Output" entry to README Table of Contents
2. Capture Events tab screenshot (5th tab, currently missing from gallery)
3. Capture a failed-step scenario (red statuses — more visually interesting than all-green)
4. Capture a retry scenario (shows attempt tracking visually in the timeline)
5. Human-verify the 4 gallery screenshots look correct (especially DAG Graph SVG rendering)
6. Add `loading="lazy"` to the hero image too (currently only gallery images have it)
7. Consider making the hero image a lightweight animated GIF showing tab switching

### Example binary improvements

8. Add `--output-dir` flag to `viz/example/main.go` to prevent clobbering tracked files
9. Add a `--screenshot-mode` flag that injects `switchTab()` calls for easier capture
10. Add deterministic mode (fixed RunID, zero timestamps) for reproducible dashboard output
11. Add `--scenario=failed` flag to generate a pipeline with failed steps for richer screenshots

### Capture pipeline

12. Document `scripts/capture-screenshots.sh` in CONTRIBUTING.md
13. Document the screenshot pipeline in AGENTS.md (under Testing Patterns or a new section)
14. Add screenshot regeneration to `flake.nix` as a devShell app
15. Convert screenshots to WebP format (40-60% size reduction)
16. Add a CI job that regenerates screenshots and fails if they drift
17. Add a test that verifies `docs/screenshots/` files exist and are non-empty
18. Add a test that README image references resolve to actual files
19. Add a link/image checker to CI for README paths
20. Add width/height to `<table>` element to prevent layout shift

### README polish

21. Replace the Mermaid code block with rendered Mermaid + a screenshot side-by-side
22. Add inline screenshots next to each diagram format section (DOT, D2, PlantUML)
23. Add screenshot of the table export (CSV/Markdown rendered)
24. Add screenshot of the ASCII tree export
25. Add OpenGraph image for social sharing (derived from dashboard screenshot)
26. Add a "Visual Output" section before "Example Output" with screenshot carousel
27. Make the gallery responsive (currently HTML table — breaks on mobile)
28. Use `<picture>` element with WebP source + PNG fallback

### Website integration

29. Add hero screenshot to `website/src/components/HeroSection.astro`
30. Add gallery to the HTML dashboard guide (`website/src/content/docs/guides/html-dashboard.mdx`)
31. Add screenshots to the export formats guide
32. Create a dedicated screenshots/visuals page on the docs site
33. Add screenshots to the landing page feature grid

### Dashboard improvements (noticed during screenshots)

34. The dashboard footer shows "Generated by workflow-auditlog" — verify branding is correct
35. Check if the Steps tab pagination renders correctly in screenshot (might be cut off)
36. Consider adding a "print mode" to the dashboard (cleaner URL bar, no scrollbars)
37. The graph SVG uses `height: 100%` which may clip in headless screenshot — test fixed height
38. Add dark/light theme toggle (screenshots only show dark theme)
39. Add a "Copy as image" button to the dashboard
40. Add PDF export of the dashboard

### Testing

41. Add a golden-file test for screenshot dimensions (prevent accidental resolution changes)
42. Benchmark screenshot generation time for CI integration
43. Add a fuzz test for the `switchTab` injection sed pattern (special chars in tab names)

### Technical debt

44. The `--virtual-time-budget` chromium flag may be deprecated in newer versions — track this
45. The `sed` injection for tab switching is fragile — consider a more robust approach
46. The capture script depends on `nix` — add a fallback path for non-NixOS users
47. Add `.gitattributes` linguist-generated for `docs/screenshots/` to exclude from language stats
48. Consider Git LFS for screenshots if the repo grows significantly
49. Add `docs/screenshots/README.md` explaining what each screenshot shows and how to regenerate
50. Track the concurrent-agent collision issue — needs a process-level fix, not just per-session awareness

---

## g) Questions (Cannot Determine Myself)

1. **Are the 4 gallery screenshots visually correct?** The DAG Graph tab in particular relies on synchronous SVG rendering via `initDAGGraph()`. The JS does pure math (Kahn's algorithm, barycenter crossing reduction, median alignment) with no DOM layout queries, so `--virtual-time-budget=10000` should render it fully. But I cannot view images to confirm nodes, edges, and labels are visible. Can you open `docs/screenshots/example-graph.png` and verify?

2. **Should the capture script use `magick` instead of `convert`?** ImageMagick 7 deprecated `convert` in favor of `magick`. The current script uses `convert` (via `nix shell nixpkgs#imagemagick`) which produces a deprecation warning. Should I switch to `magick`, or is `convert` fine for broader compatibility?

3. **Should I add `--output-dir` to the example binary?** This is the root cause of the clobbering mistake (repeated twice across two sessions). The fix is ~5 lines in `viz/example/main.go`. Should I do it now, or is it out of scope for this screenshot work?
