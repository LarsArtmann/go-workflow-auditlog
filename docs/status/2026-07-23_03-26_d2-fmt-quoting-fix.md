# Status: D2 Diagram Syntax Quoting Fix

**Date:** 2026-07-23 03:26  
**Session Scope:** Fix `d2-fmt` failures caused by invalid D2 output from go-output's D2 renderer  
**Overall Status:** FIX COMPLETE, locally verified — **BLOCKED on go-output publish + version bump**

---

## What Was The Problem?

`buildflow -s d2-fmt` failed with 23 parse errors on `dag.d2`:

```
dag.d2:8:1: unexpected map termination character } in file map
dag.d2:9:32: unexpected text after unquoted string
dag.d2:10:14: missing value after colon
...
```

**Root cause:** go-output's D2 renderer (`d2_render.go` / `d2_write.go`) used `escape.D2()` which only escaped internal string characters (`\`, `"`, `\n`, `\t`) but never wrapped values in double quotes. Two classes of values broke:

1. **Hex colors** like `#2d5a2d` — the `#` starts a comment in D2, so `style.fill: #2d5a2d` was parsed as `style.fill:` (missing value) + a comment.
2. **Labels with special chars** like `fetch [Succeeded]` — brackets and spaces are structural D2 syntax, causing map termination and unquoted string errors.

---

## a) FULLY DONE

1. **Diagnosed root cause** — traced the error from `dag.d2` through `viz.ExportD2` → `viz/d2.go` → go-output `d2_render.go`/`d2_write.go` → `escape.D2()` in `escape/escape.go`.
2. **Fixed go-output D2 renderer** — added `d2NeedsQuoting()` + `d2Quote()` to `d2/d2_render.go`. Only quotes when needed (spaces, `#`, brackets, colons, etc.); simple identifiers stay unquoted for readability and backward compatibility.
3. **Updated all call sites** — replaced every `escape.D2()` call in `d2_render.go` (9 sites) and `d2_write.go` (7 sites) with `d2Quote()`. Removed now-unused `escape` import from `d2_write.go`.
4. **Updated go-output tests** — fixed 7 assertions across `d2_convert_test.go`, `d2_node_test.go`, `d2_test.go` that expected unquoted multi-word labels.
5. **Added regression test** — `TestD2ValueQuoting` in `d2_node_test.go` with 5 cases: hex color quoting, label-with-space quoting, label-with-brackets quoting, simple label NOT quoted, named color NOT quoted.
6. **Wired local go-output** — added all 14 go-output sub-modules to `go.work` (gitignored, local dev only).
7. **Regenerated dag.d2** — ran `go run ./viz/example --export`; output now has properly quoted labels and colors.
8. **Verified d2 fmt** — both `dag.d2` and `docs/planning/execution-plan.d2` pass `d2 fmt` with zero errors.
9. **Ran full test suites** — go-output (d2, escape, integration) all pass; go-workflow-auditlog (core + viz) all pass with `-race`; fuzz tests pass (1.5M+ execs on `FuzzDiagramSpecialChars`).
10. **Verified go vet** — clean on both projects.

---

## b) PARTIALLY DONE

1. **go-output fix is committed to working tree but NOT pushed or versioned** — the changes are in `/home/lars/projects/go-output` (5 files modified, uncommitted). This fix needs to be: committed, tagged as a new version (v0.30.5 or v0.31.0), pushed, and then `viz/go.mod` needs to be bumped. Without this, the fix only works on this machine via the local `go.work` replace.
2. **go.work is gitignored** — so the workspace wiring that makes the fix work locally is invisible to CI and other developers. This is correct behavior (go.work should be local), but it means the fix is currently local-only.
3. **dag.d2 is a generated artifact** — it IS in `.gitignore` (line 59), so the regenerated file won't be committed. The committed docs that embed or reference `dag.d2` content may show the old broken version if they were checked in. Need to verify docs reference accuracy.

---

## c) NOT STARTED

1. **Publish go-output fix** — commit, tag, push to `github.com/larsartmann/go-output`.
2. **Bump viz/go.mod** — update all go-output dependencies from v0.30.4 to the new version.
3. **Clean up go.work.sum** — the workspace sum file may be stale after adding 14 modules.
4. **Update go-output CHANGELOG.md** — document the quoting fix.
5. **Run golangci-lint** — did not lint either project (only ran `go vet`).
6. **Fuzz test the quoting function itself** — `d2NeedsQuoting` / `d2Quote` could benefit from a dedicated fuzz target that verifies the function never produces invalid D2 for any input.
7. **Update other diagram renderers** — DOT, PlantUML, and Mermaid may have similar quoting issues (their escape functions are also char-only, not wrapping). DOT uses `escape.DOT` which is identical to `escape.D2`. Need to verify they don't have the same class of bug.

---

## d) TOTALLY FUCKED UP

Nothing. No data loss, no broken tests, no reverted work. The fix is correct and all tests pass.

**One thing I should have caught sooner:** I initially checked whether dag.d2 was tracked using `git ls-files dag.d2` which returned `dag.d2` — I misinterpreted this as "tracked." It was actually matching the `.gitignore` entry name, not confirming tracking. The file is NOT tracked (confirmed via `git ls-files --stage dag.d2` returning empty and `git show HEAD:dag.d2` failing). This confusion cost me one unnecessary investigation step but caused no harm.

---

## e) WHAT WE SHOULD IMPROVE

1. **go-output should have a `d2 fmt` integration test** — the library generates D2 but never validates the output is parseable by the actual `d2` CLI. A golden test that runs `d2 fmt` on rendered output would have caught this immediately.
2. **go-output escape functions are inconsistently designed** — `escape.D2()` does char-level escaping but NOT quoting. `escape.MermaidText()` does char-level replacement. `escape.SlugifyID()` does sanitization. None of them wrap in quotes. The correct pattern should be: escape chars AND conditionally quote. The quoting decision belongs in the escape function, not scattered across 16 render call sites.
3. **dag.d2 is generated by the example binary but not by any build step** — it's a manual `go run ./viz/example --export` invocation. The buildflow `d2-fmt` check runs on files that are generated ad-hoc, creating a disconnect between generation and validation.
4. **The `d2Quote` function is in the `d2` package, not `escape`** — this is the right call (quoting rules are D2-specific, not generic), but the `escape.D2` function is now misleadingly named since it does NOT produce D2-safe output on its own (it misses quoting). Consider renaming or documenting.
5. **I didn't check other renderers for the same bug** — `escape.DOT()` is literally `escape.D2()`, so DOT output likely has the same issue with unquoted hex colors and labels with spaces. DOT uses `<` for HTML labels and `#` for comments in some contexts. This is a latent bug.
6. **The `title` block in D2 output has an unquoted label** — `label: data-pipeline` works because `data-pipeline` is a valid D2 identifier (hyphens allowed). But if the WorkflowID contained a space or special char, it would break. `d2Quote` is applied via the `d2DiagramTitle` → `SetTitle` → `writeConfig` path, so this IS handled. Verified: `data-pipeline` doesn't trigger quoting because `-` is not in the `d2NeedsQuoting` character set. This is correct for D2 (hyphens are valid in keys/identifiers).

---

## f) NEXT ACTIONS (Prioritized)

### P0 — Ship the fix

1. **Commit go-output changes** — 5 files in `/home/lars/projects/go-output` (d2_render.go, d2_write.go, d2_convert_test.go, d2_node_test.go, d2_test.go)
2. **Tag go-output** — `v0.30.5` (patch: quoting fix) or `v0.31.0` (if there are other unreleased changes)
3. **Push go-output** to remote
4. **Bump viz/go.mod** — update all 12 go-output dependencies to the new version
5. **Run `go work sync`** — sync go.work.sum after version bump
6. **Verify standalone build** — `cd viz && GOWORK=off GOEXPERIMENT=jsonv2 go test ./...`
7. **Commit auditlog changes** — go.mod/go.sum bumps + dag.d2 regeneration

### P1 — Harden the fix

8. **Add `d2 fmt` validation test to go-output** — integration test that pipes rendered D2 through the `d2` CLI and asserts zero errors
9. **Audit DOT renderer** — `escape.DOT` is identical to `escape.D2`; verify DOT output doesn't have the same quoting bug for hex colors and labels with spaces
10. **Audit PlantUML renderer** — check for similar quoting gaps
11. **Audit Mermaid renderer** — check for similar quoting gaps
12. **Add fuzz target for `d2Quote`** — verify it never produces invalid D2 for arbitrary input
13. **Run golangci-lint** on both projects — `cd go-output && golangci-lint run ./d2/...` and `cd go-workflow-auditlog && golangci-lint run ./...`

### P2 — Improve the architecture

14. **Redesign escape functions** — `escape.D2()` should either be renamed to `escape.D2Chars()` or deprecated in favor of `d2.Quote()` which handles both escaping and quoting
15. **Add golden test for hex color output** — render a diagram with hex fill colors and compare against a `.golden` file
16. **Consider a `d2 fmt` CI step in go-output** — run `d2 fmt` on all test golden files as a CI gate
17. **Document the quoting behavior** in go-output's D2 package doc comment
18. **Add `dag.d2` to a Makefile/flake target** — so `nix run .#generate-diagrams` produces it deterministically
19. **Add more `d2Quote` edge cases to regression test** — empty string, only-spaces, unicode, control characters, long strings
20. **Verify `d2 fmt` doesn't modify quoted output** — run `d2 fmt` on the go-output golden test files to ensure they're stable

### P3 — Polish

21. **Update go-output CHANGELOG.md** with the quoting fix
22. **Update go-output README.md** if D2 quoting behavior is documented
23. **Add a test that verifies `d2Quote` is idempotent** — `d2Quote(d2Quote(s))` should equal `d2Quote(s)` for simple inputs (might NOT hold for inputs with quotes — verify)
24. **Check if `execution-plan.d2` needs any updates** — it was hand-written and already valid, but the go-output changes don't affect it
25. **Consider whether `title: { label: ... }` should also use `d2Quote`** — it does via `writeConfig`, but verify the title label quoting handles all edge cases
26. **Run the full go-workflow-auditlog fuzz suite** — `FuzzDiagramSanitization_MultiStep` with longer fuzztime
27. **Check if any other projects depend on go-output v0.30.4** — they may need version bumps too
28. **Add a migration note** for consumers who parse D2 output programmatically (quoted values change the string format)
29. **Review the `d2NeedsQuoting` character set** — are there other D2-special characters I missed? (e.g., `<`, `>`, `@`, `!`, `%`, `&`, `+`, `=`)
30. **Consider D2 reserved keywords** — `true`, `false`, `null`, `direction`, `shape`, `style`, `classes`, `title`, `layout` as values might need quoting to disambiguate from keywords
31. **Run `go mod tidy` on both modules** after the version bump
32. **Verify the `daghtml` adapter** — it uses `go-output/daghtml` which may also render D2-like syntax
33. **Check if the table export format `d2` is affected** — `output.RenderTableData` dispatches to D2 format; verify table-to-D2 also quotes properly
34. **Update the `execution-plan.d2` if go-output's formatter changes it** — run `d2 fmt` on it post-fix
35. **Add `d2 fmt` to the flake.nix treefmt config** — so it runs automatically on all `.d2` files (currently treefmt only has nixfmt + gofmt)
36. **Review whether the `#` comment character in D2 is version-dependent** — different D2 versions may handle it differently
37. **Add a test for nested node quoting** — `writeNestedNode` uses `d2Quote` but the regression test doesn't cover nested nodes
38. **Add a test for edge label quoting** — `writeEdge` uses `d2Quote` for labels but the regression test doesn't cover edge labels with special chars
39. **Add a test for class name quoting** — `writeClasses` uses `d2Quote` for class names
40. **Add a test for table column quoting** — `writeColumn` uses `d2Quote` for column names and types
41. **Verify the `near` attribute quoting** — `writeNodeLayout` uses `d2Quote` for `near` values
42. **Verify the `icon`/`link`/`tooltip` quoting** — all use `d2Quote` now
43. **Check if `direction` value needs quoting** — `writeConfig` outputs `direction: down` unquoted; verify this is always safe
44. **Add a test for the `layout` value quoting** — `writeConfig` uses `d2Quote` for layout engine names
45. **Consider adding `d2 fmt` as a pre-commit hook** in go-output
46. **Review whether `text-transform` value needs quoting** — it's an enum value like `uppercase`; verify enum values are always safe unquoted
47. **Document the `d2Quote` function** — explain which characters trigger quoting and why
48. **Consider a `Strict` mode** — quote everything, even simple identifiers, for maximum safety
49. **Add benchmark for `d2Quote`** — ensure the quoting check doesn't add meaningful overhead to diagram rendering
50. **Review the session's go.work changes** — clean up the workspace file or leave it for local dev (currently gitignored, so safe either way)

---

## g) QUESTIONS (Cannot Determine Myself)

1. **Should I commit and publish the go-output fix now (v0.30.5 patch), or batch it with other pending go-output changes for a v0.31.0 release?** The fix is ready to ship, but I don't know if there are other unreleased changes in the go-output working tree that should go in the same release.

2. **Should `d2-fmt` be added to the `flake.nix` treefmt configuration for this project?** Currently treefmt only runs `nixfmt` + `gofmt`. Adding `d2-fmt` would catch future D2 syntax issues automatically, but I don't know if you want D2 formatting enforced as a build gate.

3. **Should the DOT/PlantUML/Mermaid renderers be audited and fixed in the same go-output release, or shipped separately?** They likely have the same class of quoting bug (`escape.DOT` is literally `escape.D2`), but fixing all renderers at once is a bigger change with more test updates.
