---
name: release
description: >-
  Use whenever it's time for a new release of go-workflow-auditlog; mostly on
  direct request of the user saying "release", "new version", "cut a release",
  "is it time for a release", "ship it", "tag and release", or similar. Covers
  the full multi-module monorepo release process: version assessment, CHANGELOG
  preparation, go.mod version bumps, tag creation, push, GitHub Release, demo
  binaries, pkg.go.dev probing, and post-release verification. Also triggers
  when the user asks "should we release" or "what's changed since last release".
---

# Release: go-workflow-auditlog

Cut a release of this **three-module Go monorepo** (core, viz, live).
Every release produces **three annotated git tags** at the same commit.

This skill encodes lessons from v0.1.0 through v0.9.0 — including two
ad-hoc releases (v0.8.1, v0.8.2) that bypassed the process and shipped
broken sub-module go.mod files. Follow it precisely.

## Project context

| Module | Import path | Tag format | go.mod file |
|--------|-------------|------------|-------------|
| Core | `github.com/larsartmann/go-workflow-auditlog` | `vX.Y.Z` | `./go.mod` |
| Visualization | `github.com/larsartmann/go-workflow-auditlog/viz` | `viz/vX.Y.Z` | `./viz/go.mod` |
| Live | `github.com/larsartmann/go-workflow-auditlog/live` | `live/vX.Y.Z` | `./live/go.mod` |

- **Go**: 1.26.5+ (requires `GOEXPERIMENT=jsonv2` for all Go commands)
- **Stability**: ALPHA (pre-1.0) — breaking changes permitted in minor releases per `STABILITY.md`
- **Workspace**: `go.work` links all three modules locally; consumers use `GOWORK=off`
- **External deps**: `go-output` v0.35.0+, `go-sse` v0.4.0, `go-error-family` v0.10.0

---

## Phase 0: Assess Whether a Release Is Needed

Before doing anything, determine if there's enough unreleased work to justify
a release.

```bash
# Latest tags
git tag --sort=-creatordate | head -5

# Commits since last release
LAST_TAG=$(git tag --sort=-creatordate | grep -E '^v[0-9]' | head -1)
git log "${LAST_TAG}..HEAD" --oneline

# Count by type
echo "Features: $(git log "${LAST_TAG}..HEAD" --oneline | grep -c 'feat')"
echo "Fixes:    $(git log "${LAST_TAG}..HEAD" --oneline | grep -c 'fix')"
echo "Total:    $(git log "${LAST_TAG}..HEAD" --oneline | wc -l)"
```

**Recommend a release when** there are feature commits (`feat:`), breaking
changes, or significant fixes since the last tag. If only docs/chore commits
exist, tell the user it's not worth a release yet.

---

## Phase 1: Determine the Version Bump

Read `STABILITY.md` to classify the changes, then apply SemVer:

| Bump | When | Example |
|------|------|---------|
| **PATCH** (`v0.8.2`) | Bug fixes, dep bumps, toolchain/security fixes, additive Evolving API changes | v0.8.1 → v0.8.2 |
| **MINOR** (`v0.9.0`) | New features, new API additions, breaking changes (permitted in 0.x) | v0.8.2 → v0.9.0 |
| **MAJOR** (`v1.0.0`) | Post-1.0 breaking changes (not applicable yet) | — |

**Breaking changes in 0.x are MINOR bumps.** The project is ALPHA — breaking
changes are permitted between minor releases. See `STABILITY.md` for which
surfaces are Stable (require major bump) vs Evolving (may change in 0.x).

State the version number to the user before proceeding.

---

## Phase 2: Prepare the CHANGELOG

### 2.1 Check for CHANGELOG drift

Previous releases (v0.8.1, v0.8.2) shipped without CHANGELOG updates. Always
verify the `[Unreleased]` section matches what's actually been released.

```bash
# Check if the last release tag has a matching CHANGELOG section
grep '## \[' CHANGELOG.md | head -10
```

If the last release is missing a section, create one by splitting `[Unreleased]`
— move items that shipped in the last release into a new `[X.Y.Z] - YYYY-MM-DD`
section.

### 2.2 Create the new version section

Move `[Unreleased]` entries to a new `[X.Y.Z] - YYYY-MM-DD` section. Use
[Keep a Changelog](https://keepachangelog.com/) categories:

- **Added** — new features, new API surfaces
- **Changed** — changes to existing functionality (include JSON key changes!)
- **Removed** — removed features (call out breaking changes with migration notes)
- **Fixed** — bug fixes

Leave `[Unreleased]` with empty `### Added` / `### Fixed` placeholders.

### 2.3 Curate release notes

Write a shorter, user-focused version for the GitHub Release page. The
CHANGELOG has full detail; the release notes should highlight:

1. Breaking changes (with migration link to `docs/MIGRATION.md`)
2. Headline features (bullet points, not paragraphs)
3. Coverage stats

---

## Phase 3: Bump go.mod Versions

This is the **most error-prone step**. Read carefully.

### The chicken-and-egg problem

Sub-module `go.mod` files (`viz/go.mod`, `live/go.mod`) must declare a
`require` for the core module at the version being released. But `go mod tidy`
can't resolve that version from the Go module proxy until tags are pushed.
Running `go mod tidy` before pushing tags **will corrupt go.mod** (strips all
require blocks because nothing resolves).

**Solution**: bump the version string manually with `sed`, never run
`go mod tidy` until after tags are pushed.

### 3.1 Bump the version strings

```bash
VERSION="0.9.0"

# In viz/go.mod: change the core require line
sed -i "s|go-workflow-auditlog v[0-9].[0-9].[0-9]|go-workflow-auditlog v${VERSION}|g" viz/go.mod

# In live/go.mod: change both core and viz require lines
sed -i "s|go-workflow-auditlog v[0-9].[0-9].[0-9]|go-workflow-auditlog v${VERSION}|g" live/go.mod
sed -i "s|go-workflow-auditlog/viz v[0-9].[0-9].[0-9]|go-workflow-auditlog/viz v${VERSION}|g" live/go.mod
```

### 3.2 Verify the bump

```bash
grep 'auditlog' viz/go.mod live/go.mod
# viz/go.mod should show: github.com/larsartmann/go-workflow-auditlog v${VERSION}
# live/go.mod should show: github.com/larsartmann/go-workflow-auditlog v${VERSION}
#                         github.com/larsartmann/go-workflow-auditlog/viz v${VERSION}
```

### 3.3 CRITICAL: Verify no replace directives

```bash
grep '^replace' viz/go.mod live/go.mod go.mod
# MUST return nothing. replace directives in sub-modules produce
# pseudo-version requirements (v0.0.0-00010101000000-000000000000) that
# completely break consumer go get.
```

### 3.4 Do NOT run `go mod tidy` yet

go.sum files will remain stale (old version checksums). This is expected.
They will be fixed in Phase 6 after tags are pushed. **Never run
`go mod tidy` or `go mod tidy -e` on the sub-modules before tags exist on
the remote** — it will strip all require blocks and produce a broken commit.

---

## Phase 4: Pre-push Verification

Verify in **workspace mode** (using `go.work` — NOT `GOWORK=off`). The
workspace resolves all three modules from local directories, so the
unpublished v0.9.0 tag doesn't matter.

### 4.1 Core module

```bash
GOEXPERIMENT=jsonv2 go vet ./...
GOEXPERIMENT=jsonv2 go test -race -count=1 ./...
golangci-lint run --timeout=10m ./...
```

### 4.2 Viz module (workspace mode)

```bash
GOEXPERIMENT=jsonv2 go vet ./viz/...
GOEXPERIMENT=jsonv2 go test -race -count=1 ./viz/...
```

### 4.3 Live module (workspace mode)

```bash
GOEXPERIMENT=jsonv2 go vet ./live/...
GOEXPERIMENT=jsonv2 go test -race -count=1 ./live/...
```

### 4.4 Coverage check

```bash
GOEXPERIMENT=jsonv2 go test -race -coverprofile=cover.out -covermode=atomic -coverpkg=./ ./...
go tool cover -func=cover.out | tail -1
# Must be >= 92%
```

### 4.5 Git hygiene

```bash
git status   # should be clean (auto-commit daemon may need a moment)
```

If the tree is dirty, wait for the auto-commit daemon or commit manually.

**Note**: `nix run .#check` will FAIL at this point because it tests
sub-modules in `GOWORK=off` mode, which requires the published tag. This is
expected — `nix run .#check` can only pass after Phase 6. Use workspace-mode
verification (above) for pre-push confidence.

---

## Phase 5: Commit, Tag, and Push

### 5.1 Commit the release prep

```bash
git add CHANGELOG.md viz/go.mod viz/go.sum live/go.mod live/go.sum
git commit -m "chore(release): prepare v${VERSION} — bump sub-module deps and CHANGELOG"
```

If the pre-commit hook (BuildFlow) fails on missing binaries (dprint,
go-licenses, pnpm, tailwindcss, tsc, vulnix), use `--no-verify` — these are
environment issues, not code quality failures. All Go checks (vet, lint,
test-race) ran in Phase 4.

### 5.2 Create three annotated tags

```bash
COMMIT=$(git rev-parse HEAD)

git tag -a "v${VERSION}"         -m "Release v${VERSION}"         "${COMMIT}"
git tag -a "viz/v${VERSION}"     -m "Release viz/v${VERSION}"     "${COMMIT}"
git tag -a "live/v${VERSION}"    -m "Release live/v${VERSION}"    "${COMMIT}"
```

Verify:

```bash
git tag --points-at HEAD
# Should show all three tags
```

### 5.3 Push

```bash
git push origin master
git push origin "v${VERSION}" "viz/v${VERSION}" "live/v${VERSION}"
```

The Go module proxy discovers new versions from git tags automatically.
Wait 2-10 minutes for `proxy.golang.org` and `sum.golang.org` to propagate.

If `sum.golang.org` returns 500, it's checksum DB propagation delay. Wait;
resolves automatically. Test with `GOSUMDB=off` in the meantime.

---

## Phase 6: Fix go.sum Checksums (Post-push)

Now that tags are on the remote, the Go module proxy can resolve the new
version. Run `go mod tidy -e` on the sub-modules:

```bash
cd viz && GOWORK=off GOEXPERIMENT=jsonv2 go mod tidy -e && cd ..
cd live && GOWORK=off GOEXPERIMENT=jsonv2 go mod tidy -e && cd ..
```

The `-e` flag is required because go-output's published go.mod ships broken
`replace => ./testhelpers` directives that resolve to an invalid zero version.
This is the "go-output testhelpers defect" — harmless, `-e` proceeds past it.

**Verify the require blocks are intact** (tidy should NOT have stripped them):

```bash
grep 'auditlog v' viz/go.mod   # should show v${VERSION}
grep 'auditlog' live/go.mod    # should show v${VERSION} for both core and viz
```

If tidy stripped the require blocks (chicken-and-egg still biting), restore
from git and manually add the checksum lines. Then commit the go.sum update:

```bash
git add viz/go.sum live/go.sum
git commit --amend --no-edit
git push origin master --force-with-lease
# Re-tag if amended (delete + recreate tags at new HEAD)
```

### 6.1 Verify standalone builds

```bash
cd viz  && GOWORK=off GOEXPERIMENT=jsonv2 go test -count=1 ./... && cd ..
cd live && GOWORK=off GOEXPERIMENT=jsonv2 go test -count=1 ./... && cd ..
```

### 6.2 Full nix check

```bash
nix run .#check
# Now all three modules should pass standalone (GOWORK=off) mode.
```

---

## Phase 7: Create the GitHub Release

### Option A: goreleaser (preferred — builds binaries + checksums)

**Prerequisite**: clean working tree.

```bash
GORELEASER_CURRENT_TAG="v${VERSION}" \
GITHUB_TOKEN="$(gh auth token)" \
GOEXPERIMENT=jsonv2 \
  goreleaser release --clean --release-notes /tmp/release-notes.md
```

**Critical goreleaser gotchas**:

- `GORELEASER_CURRENT_TAG` is **required** — three tags share one commit and
  goreleaser's `git describe` picks the alphabetically-last (`live/v*`) without
  the override. Always pass the CORE tag.
- Before-hooks are wrapped in `sh -c "..."` because goreleaser OSS uses
  direct `exec.CommandContext` (not a shell).
- Sub-module hooks use `go mod tidy -e` (go-output testhelpers defect).

### Option B: gh CLI (fallback — when tree is dirty or goreleaser unavailable)

```bash
gh release create "v${VERSION}" \
  --title "v${VERSION}" \
  --notes-file /tmp/release-notes.md \
  --latest --prerelease
```

All v0.x releases are `--prerelease` (the library is pre-1.0 ALPHA).

---

## Phase 8: Build Demo Binaries (if using gh CLI)

Cross-platform demo binary from `./viz/example`:

```bash
VERSION="0.9.0"
BUILD_DIR="/tmp/demo-builds"
rm -rf "$BUILD_DIR" && mkdir -p "$BUILD_DIR"

for pair in "linux_amd64" "linux_arm64" "darwin_amd64" "darwin_arm64"; do
  os="${pair%%_*}"; arch="${pair#*_}"
  os_title="$(echo "$os" | sed 's/.*/\u&/')"
  arch_title="$(echo "$arch" | sed 's/.*/\u&/')"
  archive_name="go-workflow-auditlog_${VERSION}_${os_title}_${arch_title}"
  dir="$BUILD_DIR/$archive_name"
  mkdir -p "$dir"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 GOEXPERIMENT=jsonv2 \
    go build -o "$dir/workflow-auditlog-demo" \
    -ldflags "-s -w -X main.version=${VERSION}" \
    ./viz/example
  cp README.md LICENSE "$dir/" 2>/dev/null
  tar -C "$BUILD_DIR" -czf "$BUILD_DIR/${archive_name}.tar.gz" "$archive_name"
done

cd "$BUILD_DIR" && sha256sum *.tar.gz > checksums.txt

gh release upload "v${VERSION}" "$BUILD_DIR"/*.tar.gz "$BUILD_DIR/checksums.txt" --clobber
```

---

## Phase 9: Probe pkg.go.dev and Verify go get

### 9.1 Trigger pkg.go.dev doc generation

Use the `fetch` tool (NOT `curl` — it's banned in Crush):

```
https://pkg.go.dev/fetch/github.com/larsartmann/go-workflow-auditlog@v${VERSION}
https://pkg.go.dev/fetch/github.com/larsartmann/go-workflow-auditlog/viz@viz/v${VERSION}
https://pkg.go.dev/fetch/github.com/larsartmann/go-workflow-auditlog/live@live/v${VERSION}
```

### 9.2 Verify go get works in clean directories

```bash
# Core
rm -rf /tmp/test-core && mkdir /tmp/test-core && cd /tmp/test-core
go mod init test && GOEXPERIMENT=jsonv2 go get github.com/larsartmann/go-workflow-auditlog@v${VERSION}

# Viz (standalone — proves no replace-directive leak)
rm -rf /tmp/test-viz && mkdir /tmp/test-viz && cd /tmp/test-viz
go mod init test && GOEXPERIMENT=jsonv2 go get github.com/larsartmann/go-workflow-auditlog/viz@viz/v${VERSION}

# Live (standalone — proves no replace-directive leak)
rm -rf /tmp/test-live && mkdir /tmp/test-live && cd /tmp/test-live
go mod init test && GOEXPERIMENT=jsonv2 go get github.com/larsartmann/go-workflow-auditlog/live@live/v${VERSION}
```

If any `go get` fails with `unknown revision 000000000000`, the sub-module
go.mod has a `replace` directive that wasn't caught in Phase 3. Fix it,
amend, re-tag, re-push.

### 9.3 Check CI

Verify CI is green on master after the tag push.

---

## Phase 10: Post-release Cleanup

### 10.1 CHANGELOG

Ensure `[Unreleased]` section exists with empty placeholders:

```markdown
## [Unreleased]

### Added

- Nothing yet.

### Fixed

- Nothing yet.
```

### 10.2 Documentation sync

Check whether these need updating for the new release:

- `STABILITY.md` — new API surfaces classified as Stable or Evolving
- `AGENTS.md` — source file inventory, test counts, coverage numbers
- `FEATURES.md` — feature status updates
- `docs/MIGRATION.md` — breaking change migration guides

### 10.3 Release verification summary

Confirm to the user:
- GitHub Release URL
- All three pkg.go.dev pages render
- `go get` works for all three modules
- CI is green
- Coverage meets the 92% gate

---

## Quick Reference: Gotchas

| Issue | Cause | Fix |
|-------|-------|-----|
| `go mod tidy` strips all require blocks | Tags not pushed yet; proxy can't resolve version | Never run tidy before push. Bump versions with `sed`. |
| Consumer `go get` fails with `unknown revision 000000000000` | Sub-module go.mod has `replace` directive | Remove `replace`. Use real version in `require`. |
| `go mod tidy` fails on viz/live | go-output's go.mod has broken `replace => ./testhelpers` | Use `go mod tidy -e` (error-tolerant) |
| goreleaser picks wrong tag | Three tags at same commit; `git describe` returns `live/v*` | Set `GORELEASER_CURRENT_TAG=vX.Y.Z` |
| goreleaser hooks fail silently | OSS hooks use direct exec, not shell | Wrap hooks in `sh -c "..."` |
| goreleaser "dirty state" | Auto-commit daemon hasn't committed | Wait for daemon, or use `gh release create` |
| `sum.golang.org` returns 500 | Checksum DB propagation delay | Wait; test with `GOSUMDB=off` |
| Local tags break workspace builds | go.work uses filesystem, not tags; local tags force proxy resolution | Don't create tags until ready to push |
| CHANGELOG missing a release section | Ad-hoc release bypassed CHANGELOG | Always update CHANGELOG in Phase 2 |
| Pre-commit hook fails on missing binaries | dprint, pnpm, tsc, etc. not in nix devShell | Use `--no-verify` (infra failures only, not code) |
| `nix run .#check` fails pre-push | Tests sub-modules in GOWORK=off (needs published tag) | Verify in workspace mode pre-push; run nix check post-push |

---

## Checklist Summary

```
[ ] Phase 0: Assess — enough unreleased work?
[ ] Phase 1: Determine version bump (PATCH / MINOR)
[ ] Phase 2: CHANGELOG — split [Unreleased], create [X.Y.Z], curate notes
[ ] Phase 3: Bump go.mod versions with sed (NOT tidy)
[ ] Phase 3: Verify no replace directives
[ ] Phase 4: Vet + test + lint all modules (workspace mode)
[ ] Phase 4: Coverage >= 92%
[ ] Phase 5: Commit release prep
[ ] Phase 5: Create three annotated tags at same commit
[ ] Phase 5: Push master + three tags
[ ] Phase 6: go mod tidy -e on viz/live (post-push)
[ ] Phase 6: Verify standalone builds pass
[ ] Phase 6: nix run .#check passes
[ ] Phase 7: Create GitHub Release (goreleaser or gh CLI)
[ ] Phase 8: Build + upload demo binaries (if gh CLI path)
[ ] Phase 9: Probe pkg.go.dev for all three modules
[ ] Phase 9: Verify go get in clean directories
[ ] Phase 9: CI green on master
[ ] Phase 10: CHANGELOG [Unreleased] has empty placeholders
[ ] Phase 10: Docs updated (STABILITY.md, AGENTS.md, FEATURES.md)
```
