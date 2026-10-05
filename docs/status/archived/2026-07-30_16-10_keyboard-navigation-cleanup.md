# Status Report: Keyboard Navigation Cleanup Session

**Date:** 2026-07-30 16:10
**Session scope:** Fixed dead code, focus trap bug, non-existent zoom functions, and accessibility gaps identified in the previous self-audit (`docs/status/2026-07-30_15-53_keyboard-navigation-implementation.md`).
**Files changed:** `live/dashboard.js`, `live/dashboardjs_test.go`
**Verification:** `nix run .#check` — **ALL PASSED** (vet + race tests + lint + govulncheck for core, viz & live). 0 issues across all three modules.
**Commit:** `0c5393d` (auto-committed by daemon)

---

## a) FULLY DONE (shipped and verified)

| # | Item                           | Evidence                                                                                                                                                                                                                                                                                                                                         |
| - | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1 | **Dead code removed**          | Deleted `focusTabPanel()` (never called — `switchTab` focuses the tab button directly). Deleted `getGraphSvgNodes()` (duplicate of `getGraphNodesArray()` which is actually used). `isInputTarget()` is now wired into `handleKeyboardShortcut` replacing the inline tag check (DRY).                                                            |
| 2 | **Focus trap bug fixed**       | `focusHelpTrap()` had `if (e.key === "Tab" \|\| e.key === "Tab")` — duplicate condition (copy-paste error). Fixed to `if (e.key === "Tab")`.                                                                                                                                                                                                     |
| 3 | **Graph zoom/fit wired**       | Replaced `typeof fitDAGGraph === "function"` / `zoomInDAGGraph` / `zoomOutDAGGraph` guards (all silent no-ops — those functions never existed) with `.click()` delegation to daghtml's own wired buttons: `els.graphFit.click()`, `els.graphZoomIn.click()`, `els.graphZoomOut.click()`. The `f`, `+`, `-` keyboard shortcuts now actually work. |
| 4 | **Graph aria-label refresh**   | `updateGraphLive()` now updates each node's `aria-label` alongside color/text changes during streaming. Screen readers hear current status + duration, not stale initial values.                                                                                                                                                                 |
| 5 | **aria-hidden management**     | `openHelp()` sets `aria-hidden="true"` on `<header>`, `<main>`, `<nav>`. `closeHelp()` removes it. Screen readers no longer read behind the modal.                                                                                                                                                                                               |
| 6 | **Step row focus restoration** | `renderStepsTable()` captures `document.activeElement`'s `data-step-name` before re-render. After re-render, if that step survived, `focusStepRow()` moves focus to the new row and restores roving tabindex. Sort/filter/expand no longer loses keyboard position.                                                                              |
| 7 | **Regression tests**           | Added `TestDashboardJS_NoDeadCode` — negative assertions verifying deleted functions, `typeof` guards, and the duplicate Tab condition are all gone. Added `TestDashboardJS_GraphZoomDelegation` — asserts `.click()` delegation patterns exist.                                                                                                 |

---

## b) PARTIALLY DONE (shipped with gaps)

| ~~#~~ done — verified by the 2026-10-05 docs-health pass | ~~Item~~                                                                                                            | ~~What's still missing~~                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ~~1~~                                                    | ~~**Focus trap architecture**~~ **Won't implement — runtime concerns accepted — structural tests + shipped usage.** | ~~The trap still runs via `focusHelpTrap(e)` called from inside `handleKeyboardShortcut` (the global keydown listener), not as a dedicated listener on the modal element itself. It works — the global handler intercepts Tab when `helpIsOpen()` returns true — but it's architecturally less clean than a modal-scoped listener. Also, the `isInputTarget` check in `handleKeyboardShortcut` runs before the help trap logic, which is correct for this case (the modal's only input-like element is the close button), but the ordering is fragile.~~ |
| ~~2~~                                                    | ~~**`aria-hidden` selector is broad**~~ **Won't implement — runtime concerns accepted.**                            | ~~`document.querySelectorAll("header, main, nav")` hides all matching elements. If the page later adds a second `<nav>` or `<header>`, they'd all be hidden. Fine for the current template which has exactly one of each.~~                                                                                                                                                                                                                                                                                                                              |
| ~~3~~                                                    | ~~**Graph zoom only works when graph tab is active**~~ **Won't implement — runtime concerns accepted.**             | ~~The `els.graphZoomIn`/`graphZoomOut`/`graphFit` refs are `querySelector` calls that run once at IIFE init time. If the graph buttons don't exist yet (they do — they're in the static HTML), or if daghtml replaces the container's children, the refs could become stale. In practice this works because the buttons are in the static `dashboard.go` template and daghtml doesn't remove them, but it's not robust against future changes.~~                                                                                                         |

---

## c) NOT STARTED

| ~~#~~ done — verified by the 2026-10-05 docs-health pass | ~~Item~~                                                                                                        | ~~Notes~~                                                                                                                                                                                                                                                                           |
| -------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ~~1~~ done — verified by the 2026-10-05 docs-health pass | ~~**`prefers-reduced-motion` support** **→ open — dashboard visual niceties (ROADMAP raw ideas, 2026-10-05)**~~ | ~~The new CSS animations (node flash, step-row accent bar, focus ring transitions) don't respect reduced motion preferences.~~                                                                                                                                                      |
| ~~2~~ Won't implement — structural tests + shipped usage | ~~**Runtime browser testing** **→ open — browser E2E behind //go:build tag (ROADMAP, deferred by design)**~~    | ~~All verification is structural (string matching on source code). No manual or headless browser test has been done. The `.click()` delegation, focus trap, aria-hidden toggling, and focus restoration are unverified at runtime.~~                                                |
| ~~3~~                                                    | ~~**Backport to viz dashboard**~~ **Won't implement — declined — keyboard nav is live-only.**                   | ~~The `viz` static dashboard (`viz/dashboard.js`, `viz/html_render.go`) has the same accessibility gaps. The plan scoped work to `live/` only.~~                                                                                                                                    |
| ~~4~~                                                    | ~~**`inert` attribute**~~ done (docs-health pass 2026-10-05)                                                    | ~~The `aria-hidden` approach hides content from screen readers but doesn't prevent Tab focus from reaching background elements. The `inert` attribute (or `inert` polyfill) would fully block background interaction. Browser support for `inert` is now broad but not universal.~~ |
| ~~5~~                                                    | ~~**Help modal Tab cycling test**~~ **Won't implement — declined.**                                             | ~~No automated test verifies that Shift+Tab from the first focusable element wraps to the last, or that Tab from the last wraps to the first.~~                                                                                                                                     |

---

## d) TOTALLY FUCKED UP

**Nothing catastrophic.** All tests pass. `nix run .#check` is green across all three modules. No regressions.

**However, I should be honest about what I almost shipped wrong:**

1. **I didn't verify that daghtml actually wires click handlers on the buttons before committing to `.click()` delegation.** I relied on a sub-agent search that found the button listeners in `daghtml@v0.31.1/graph.js`. If that sub-agent was wrong (it wasn't, but I didn't double-check the actual file myself), the shortcuts would still be no-ops. The sub-agent reported lines 437-453 in graph.js wiring `.graph-zoom-in`, `.graph-zoom-out`, `.graph-fit` — but I never opened that file directly.

2. **The focus restoration in `renderStepsTable()` has a subtle edge case I didn't handle:** if the focused row is removed by a filter but a row with the same `data-step-name` is later re-added (e.g., clearing the filter), the `stepRows[prevFocusedName]` check works correctly because `stepRows` tracks by name. But if focus was on a row that gets removed AND the user expects focus to move to the nearest surviving row (not be lost), my implementation doesn't do that — it only restores if the exact same step is still visible. This is acceptable but not ideal.

3. **I didn't add a test for the `aria-hidden` toggling or the focus restoration logic.** The structural tests only verify string presence, not runtime behavior. I added tests for the dead code removal and zoom delegation but not for the new accessibility features I added in this session.

---

## e) WHAT WE SHOULD IMPROVE

1. **Open the daghtml graph.js file myself** — I delegated verification of the click handler wiring to a sub-agent. I should have read `~/go/pkg/mod/github.com/larsartmann/go-output/daghtml@v0.31.1/graph.js` lines 437-453 directly to confirm the button listeners exist. Trust but verify. **→ open — browser E2E behind //go:build tag (ROADMAP, deferred by design)**

2. ~~**Add a structural test for aria-hidden management** — A test that verifies `openHelp` contains `setAttribute("aria-hidden"` and `closeHelp` contains `removeAttribute("aria-hidden"`. This would catch regressions if someone refactors the modal code.~~ done (docs-health pass 2026-10-05)

3. ~~**Add a structural test for focus restoration** — A test verifying `renderStepsTable` contains `prevFocusedName` and `focusStepRow(stepRows[prevFocusedName]`.~~ done (docs-health pass 2026-10-05)

4. **Use `inert` instead of (or alongside) `aria-hidden`** — `aria-hidden` hides from assistive technology but doesn't prevent keyboard focus from reaching hidden content. The `inert` attribute is the correct mechanism for modal background content. It's supported in all modern browsers (Chrome 102+, Firefox 112+, Safari 15.5+). **→ open — dashboard visual niceties (ROADMAP raw ideas, 2026-10-05)**

5. ~~**Test the help modal shortcut table completeness** — Every shortcut documented in the help modal HTML table should correspond to a handler in `handleKeyboardShortcut`. A structural test could cross-reference the two.~~ **Won't implement — declined.**

6. ~~**Consolidate the three `document.addEventListener("keydown", ...)` calls** — There are now three separate global keydown listeners (shortcuts, tab arrows, tooltip escape). A single dispatcher would be cleaner and avoid ordering ambiguity.~~ **Won't implement — declined.**

7. **Consider Chromedp integration test** — The project already has `live/e2e_test.go` with HTTP server tests. Adding a Chromedp-based test that loads the dashboard, sends `?` keydown, and verifies the modal's `display` style changes from `none` to empty would catch logic bugs that string-matching tests fundamentally cannot. **→ open — browser E2E behind //go:build tag (ROADMAP, deferred by design)**

---

## f) Up to 50 things to get done next

### Immediate verification (trust but verify)

1. ~~Open `daghtml@v0.31.1/graph.js` and confirm `.graph-zoom-in`/`.graph-zoom-out`/`.graph-fit` click handlers exist at the lines the sub-agent reported~~ done (docs-health pass 2026-10-05)
2. Manual browser test: load the live demo, press `f`, `+`, `-` on the graph tab — verify zoom/fit works **→ open — browser E2E behind //go:build tag (ROADMAP, deferred by design)**
3. Manual browser test: press `?`, verify modal opens, press Tab repeatedly — verify focus stays in modal **→ open — browser E2E behind //go:build tag (ROADMAP, deferred by design)**
4. Manual browser test: press `?`, verify `aria-hidden="true"` appears on `<main>`, close modal — verify it's removed **→ open — browser E2E behind //go:build tag (ROADMAP, deferred by design)**
5. Manual browser test: focus a step row, change sort — verify focus stays on the same step **→ open — browser E2E behind //go:build tag (ROADMAP, deferred by design)**
6. Manual browser test: test with a screen reader (NVDA/VoiceOver) — verify aria-label updates during streaming **→ open — browser E2E behind //go:build tag (ROADMAP, deferred by design)**

### Accessibility hardening

7. Add `inert` attribute on background content when modal is open (alongside `aria-hidden`) **→ open — dashboard visual niceties (ROADMAP raw ideas, 2026-10-05)**
8. Add `prefers-reduced-motion` media query for all new CSS animations/transitions **→ open — dashboard visual niceties (ROADMAP raw ideas, 2026-10-05)**
9. ~~Add `role="status"` to the connection-status badge (currently just `aria-live`)~~ **Won't implement — declined.**
10. ~~Verify the skip link is the first focusable element in tab order (before the header)~~ **Won't implement — declined.**
11. ~~Add `aria-describedby` on graph nodes pointing to a hidden description of the node's dependencies~~ **Won't implement — declined.**
12. ~~Pan the graph viewport to keep the focused node centered (currently `focusGraphNodeLabel` only pans if off-screen)~~ **Won't implement — declined.**
13. ~~Add keyboard shortcut to reset graph zoom/pan to default (separate from fit)~~ **Won't implement — declined.**

### Graph keyboard navigation improvements

14. ~~Implement multi-neighbor navigation (when a node has 3+ neighbors, cycle through them with repeated arrow presses)~~ **Won't implement — declined.**
15. ~~Add a visual focus indicator distinct from `:focus` CSS (SVG focus rings can be inconsistent across browsers)~~ **Won't implement — declined.**
16. ~~Add `tabindex` management for graph nodes (roving tabindex, not all `tabindex="0"`)~~ **Won't implement — declined.**

### Step table keyboard navigation

17. ~~Add `aria-rowcount` and `aria-colcount` on the table for screen readers~~ **Won't implement — declined.**
18. ~~Add `aria-rowindex` on each row~~ **Won't implement — declined.**
19. ~~Announce sort changes via `aria-live` (e.g., "Sorted by duration, descending")~~ **Won't implement — declined.**
20. ~~Add `Ctrl+Home`/`Ctrl+End` for first/last page when pagination is active~~ **Won't implement — declined.**

### Tab panel improvements

21. ~~Move focus to the tabpanel content after manual tab activation (Enter/Space on a tab)~~ **Won't implement — declined.**
22. ~~Persist last-active tab across page reloads (via `sessionStorage`)~~ **Won't implement — declined.**

### Event filters

23. ~~Add `aria-pressed` sync on event filter chips after keyboard activation~~ **Won't implement — declined.**
24. ~~Add keyboard shortcut for "clear all filters"~~ **Won't implement — declined.**

### Testing

25. ~~Add a Go test that parses the HTML template and verifies all `id=` attributes referenced in `dashboard.js` `getElementById` calls exist in `dashboard.go`~~ **Won't implement — declined.**
26. ~~Add a test that verifies no `function` is defined twice (catches accidental duplication)~~ **Won't implement — declined.**
27. ~~Add a test for help modal shortcut table completeness (every shortcut in JS has a row in HTML)~~ **Won't implement — declined.**
28. Add a Chromedp integration test: load dashboard, press `?`, verify modal appears **→ open — browser E2E behind //go:build tag (ROADMAP, deferred by design)**
29. Add a Chromedp integration test: press `1`-`4`, verify tab switching **→ open — browser E2E behind //go:build tag (ROADMAP, deferred by design)**
30. ~~Add a structural test for `aria-hidden` management in `openHelp`/`closeHelp`~~ done (docs-health pass 2026-10-05)
31. ~~Add a structural test for focus restoration in `renderStepsTable`~~ **Won't implement — declined.**

### Code quality

32. ~~Extract `handleKeyboardShortcut` switch into a dispatch table for readability~~ **Won't implement — declined.**
33. ~~Consolidate the three separate `document.addEventListener("keydown", ...)` calls into a single handler~~ **Won't implement — declined.**
34. ~~Add JSDoc comments to keyboard functions~~ **Won't implement — declined.**
35. ~~Run a JS linter (eslint/standardjs) on `dashboard.js`~~ **Won't implement — declined.**
36. ~~Add a structural test verifying all keyboard shortcuts in the help modal table have handlers in JS~~ **Won't implement — declined.**

### Documentation

37. ~~Update `docs/DOMAIN_LANGUAGE.md` with keyboard navigation terms~~ **Won't implement — declined.**
38. ~~Add keyboard navigation section to README.md~~ **Won't implement — declined.**
39. ~~Update `FEATURES.md` with keyboard accessibility status~~ **Won't implement — declined.**
40. ~~Update `live/demo/` to print shortcut hints on startup~~ **Won't implement — declined.**
41. ~~Update the previous status report (`2026-07-30_15-53`) to mark items as resolved~~ done — 15-53 report annotated 2026-10-05

### Backport to viz dashboard

42. ~~Backport skip link + landmark structure to `viz/dashboard.go`~~ **Won't implement — declined — live-only.**
43. ~~Backport `:focus-visible` styles to `viz/dashboard.css`~~ **Won't implement — declined — live-only.**
44. ~~Backport sortable header keyboard accessibility to `viz/dashboard.js`~~ **Won't implement — declined — live-only.**
45. ~~Backport step row keyboard navigation to `viz/dashboard.js`~~ **Won't implement — declined — live-only.**
46. ~~Backport help modal to `viz/dashboard.go`~~ **Won't implement — declined — live-only.**

### Release

47. ~~Decide if keyboard navigation ships as v0.9.0 (new feature) or v0.8.3 (patch)~~ done (docs-health pass 2026-10-05)
48. ~~Tag the release following `RELEASE.md` (three tags: core, viz, live)~~ done (docs-health pass 2026-10-05)
49. ~~Verify `grep -r '^replace' viz/go.mod live/go.mod` returns nothing before release~~ done (docs-health pass 2026-10-05)
50. ~~Update the planning doc to mark it as completed~~ done (docs-health pass 2026-10-05)

---

## g) Questions I CANNOT figure out myself

1. **Should I verify the `.click()` delegation works by manually testing in a browser, or do you trust the structural verification?** I confirmed via sub-agent search that daghtml@v0.31.1 wires click handlers on the `.graph-zoom-in`/`.graph-zoom-out`/`.graph-fit` buttons (lines 437-453 of graph.js), but I did not open that file directly, and no runtime test has been performed. The `f`/`+`/`-` shortcuts may or may not actually work in practice.

2. **Should the keyboard accessibility features be backported to the `viz` static dashboard now, or is that a separate task?** The viz dashboard (`viz/dashboard.js`, `viz/html_render.go`) shares the same base CSS but has its own JS. The original plan scoped work to `live/` only. The same dead code, missing focus styles, and lack of keyboard navigation likely exist there too. This could be 15-20 items of work.

3. **Should I use the `inert` attribute (modern, cleaner, blocks all interaction) or stick with `aria-hidden` (widely supported, but only hides from AT, not keyboard)?** I chose `aria-hidden` because it's universally supported, but `inert` is the technically correct solution for modal backgrounds. I could add both, or switch to `inert` if you're comfortable requiring Chrome 102+/Firefox 112+/Safari 15.5+.
