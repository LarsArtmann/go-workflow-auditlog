# Release Process

Step-by-step guide for cutting a release of go-workflow-auditlog.
This is a **multi-module monorepo**: every release produces **three tags**.

---

## Tag Convention

| Module        | Import path                                        | Tag format    | Example       |
| ------------- | -------------------------------------------------- | ------------- | ------------- |
| Core          | `github.com/larsartmann/go-workflow-auditlog`      | `vX.Y.Z`      | `v0.8.1`      |
| Visualization | `github.com/larsartmann/go-workflow-auditlog/viz`  | `viz/vX.Y.Z`  | `viz/v0.8.1`  |
| Live          | `github.com/larsartmann/go-workflow-auditlog/live` | `live/vX.Y.Z` | `live/v0.8.1` |

All three tags point to the **same commit**. Sub-module path prefixes are
required by the Go module system so `go get` can resolve each module
independently.

### SemVer guidance

- **PATCH** (`v0.8.1`): bug fixes, dependency bumps, additive changes on
  Evolving API surfaces, toolchain/security fixes.
- **MINOR** (`v0.9.0`): new features, new Stable/Evolving API additions.
- **MAJOR** (`v1.0.0`): breaking changes (post-1.0 stability guarantee).

Consult [`STABILITY.md`](STABILITY.md) for which API surfaces permit
breaking changes in 0.x minor releases.

---

## Pre-release Checklist

1. **Run the canonical check suite:**

   ```bash
   nix run .#check
   ```

   This runs `go vet`, `go test -race`, `golangci-lint`, and `govulncheck`
   for **all three modules** (core, viz, live) in standalone `GOWORK=off`
   mode. Zero findings required.

2. **Verify coverage** is at or above the 92% gate:

   ```bash
   GOEXPERIMENT=jsonv2 go test -race -coverprofile=cover.out -covermode=atomic -coverpkg=./ ./...
   go tool cover -func=cover.out | tail -1
   ```

3. **Update [`CHANGELOG.md`](CHANGELOG.md):**
   - Move `[Unreleased]` entries to a new `[X.Y.Z] - YYYY-MM-DD` section.
   - Use Keep a Changelog categories: Added / Changed / Fixed / Removed.

4. **Verify standalone builds** (proves consumers can `go get`):

   ```bash
   cd viz  && GOWORK=off GOEXPERIMENT=jsonv2 go test -count=1 ./...
   cd live && GOWORK=off GOEXPERIMENT=jsonv2 go test -count=1 ./...
   ```

5. **CRITICAL: Verify no `replace` directives in sub-module go.mod files:**

   ```bash
   grep -r '^replace' viz/go.mod live/go.mod
   # MUST return nothing. replace directives produce pseudo-version requirements
   # that break consumer go get. Local dev uses go.work use directives, not replace.
   ```

6. **Commit all changes.** The auto-commit daemon handles this, but verify:
   ```bash
   git status   # should be clean before tagging
   ```

---

## Step 1 — Create Tags

All three tags at the same commit (current HEAD):

```bash
VERSION="0.8.1"
COMMIT=$(git rev-parse HEAD)

git tag -a "v${VERSION}"         -m "Release v${VERSION}"         "${COMMIT}"
git tag -a "viz/v${VERSION}"     -m "Release viz/v${VERSION}"     "${COMMIT}"
git tag -a "live/v${VERSION}"    -m "Release live/v${VERSION}"    "${COMMIT}"
```

Verify:

```bash
git tag -l --sort=-creatordate | head
git tag --points-at HEAD
```

---

## Step 2 — Push Tags

```bash
git push origin master
git push origin "v${VERSION}" "viz/v${VERSION}" "live/v${VERSION}"
```

The Go module proxy discovers new versions from git tags automatically.

---

## Step 3 — Create the GitHub Release

### Option A: goreleaser (preferred — builds demo binary + archives + checksums)

**Prerequisite:** clean working tree (coordinate with the auto-commit daemon
— wait for it to commit pending changes before proceeding).

```bash
VERSION="0.8.1"

# Extract release notes from CHANGELOG (or write a custom file).
# goreleaser's auto-generated changelog EXCLUDES chore: commits, so always
# provide curated notes via --release-notes for an accurate release page.
gh release create "v${VERSION}" ...   # see Option B if tree is dirty

# When the tree is clean:
GORELEASER_CURRENT_TAG="v${VERSION}" \
GITHUB_TOKEN="$(gh auth token)" \
GOEXPERIMENT=jsonv2 \
  goreleaser release --clean --release-notes /tmp/release-notes.md
```

**Critical goreleaser notes for this monorepo:**

- `GORELEASER_CURRENT_TAG` is **required** — three tags share one commit,
  and goreleaser's `git describe` picks the alphabetically-last one
  (`live/v*`) without the override. Always pass the core tag.
- Before-hooks are wrapped in `sh -c "..."` because goreleaser OSS uses
  direct `exec.CommandContext` (not a shell) — inline env vars and `cd`
  do not work without the wrapper.
- Sub-module hooks use `go mod tidy -e` (error-tolerant) because
  go-output's published `go.mod` ships broken `replace => ./testhelpers`
  directives. See AGENTS.md "go-output testhelpers defect".

### Option B: gh CLI (fallback — when the tree is dirty)

```bash
VERSION="0.8.1"

# Create the release
gh release create "v${VERSION}" \
  --title "v${VERSION}" \
  --notes-file release-notes.md \
  --latest --prerelease

# Build + upload demo binaries (see "Building Demo Binaries" below)
```

---

## Building Demo Binaries

Cross-platform demo binary (`./viz/example`) for users to try the library's
output without writing code:

```bash
VERSION="0.8.1"
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

# Checksums
cd "$BUILD_DIR" && sha256sum *.tar.gz > checksums.txt

# Upload
gh release upload "v${VERSION}" "$BUILD_DIR"/*.tar.gz "$BUILD_DIR/checksums.txt" --clobber
```

---

## Step 4 — Probe pkg.go.dev

After pushing tags, trigger doc generation on pkg.go.dev:

```bash
# Core module
curl -s "https://pkg.go.dev/fetch/github.com/larsartmann/go-workflow-auditlog@v${VERSION}"

# Viz sub-module
curl -s "https://pkg.go.dev/fetch/github.com/larsartmann/go-workflow-auditlog/viz@viz/v${VERSION}"

# Live sub-module
curl -s "https://pkg.go.dev/fetch/github.com/larsartmann/go-workflow-auditlog/live@live/v${VERSION}"
```

Verify the pages render:

- https://pkg.go.dev/github.com/larsartmann/go-workflow-auditlog
- https://pkg.go.dev/github.com/larsartmann/go-workflow-auditlog/viz
- https://pkg.go.dev/github.com/larsartmann/go-workflow-auditlog/live

---

## Step 5 — Post-release Verification

1. **CI is green** on master after the tag push.
2. **`go get` resolves** in a clean directory (test each independently):
   ```bash
   # Core
   cd /tmp/test-core && go mod init test && \
   GOEXPERIMENT=jsonv2 go get github.com/larsartmann/go-workflow-auditlog@v${VERSION}

   # Viz (standalone — proves no replace-directive leak)
   cd /tmp/test-viz && go mod init test && \
   GOEXPERIMENT=jsonv2 go get github.com/larsartmann/go-workflow-auditlog/viz@v${VERSION}

   # Live (standalone — proves no replace-directive leak)
   cd /tmp/test-live && go mod init test && \
   GOEXPERIMENT=jsonv2 go get github.com/larsartmann/go-workflow-auditlog/live@v${VERSION}
   ```
3. **GitHub Release** has correct assets and notes.
4. **CHANGELOG.md** has an empty `[Unreleased]` section.

---

## Common Gotchas

| Issue                                                                             | Cause                                                                                  | Fix                                                                                                           |
| --------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| **Consumer `go get` fails with `invalid version: unknown revision 000000000000`** | Sub-module `go.mod` has `replace => ..` directive producing pseudo-version requirement | **Remove the `replace` directive.** Set real version in `require`. Local dev uses `go.work` `use` directives. |
| `go mod tidy` fails on viz/live                                                   | go-output's published `go.mod` has broken `replace => ./testhelpers`                   | Use `go mod tidy -e` (error-tolerant)                                                                         |
| goreleaser picks wrong tag                                                        | Three tags at same commit; `git describe` returns `live/v*`                            | Set `GORELEASER_CURRENT_TAG=vX.Y.Z`                                                                           |
| goreleaser hooks fail with "executable not found"                                 | OSS hooks use direct exec, not shell                                                   | Wrap hooks in `sh -c "..."`                                                                                   |
| goreleaser "git is in a dirty state"                                              | Auto-commit daemon hasn't committed yet                                                | Wait for daemon, or use `gh release create`                                                                   |
| `git work sync` downloads invalid testhelpers version                             | Same go-output replace defect                                                          | Harmless; builds/tests are unaffected                                                                         |
| `sum.golang.org` returns 500 for newly-pushed tags                                | Checksum DB propagation delay (minutes to hours)                                       | Wait; resolves automatically. Test with `GOSUMDB=off` in the meantime.                                        |
