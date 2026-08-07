# Contributing

Thanks for your interest in contributing to `go-workflow-auditlog`!

## How to Contribute

1. Fork the repository and create a feature branch from `master`.
2. Make your changes with tests.
3. Ensure all checks pass (see below).
4. Submit a pull request describing the **what** and **why**.

## Development Setup

Requires Go 1.26+ with `GOEXPERIMENT=jsonv2` and [`golangci-lint` v2](https://golangci-lint.run/).

```bash
git clone https://github.com/LarsArtmann/go-workflow-auditlog.git
cd go-workflow-auditlog
GOEXPERIMENT=jsonv2 go build ./...
```

Verify your environment:

```bash
GOEXPERIMENT=jsonv2 go test -race ./...    # all tests pass
golangci-lint run ./...                      # 0 issues
```

## Commands

| Command                                                           | Purpose                                    |
| ----------------------------------------------------------------- | ------------------------------------------ |
| `GOEXPERIMENT=jsonv2 go test ./...`                               | Run core tests                             |
| `GOEXPERIMENT=jsonv2 go test -race ./...`                         | Core tests with race detector              |
| `GOEXPERIMENT=jsonv2 go test -race -coverprofile=cover.out ./...` | Core tests with coverage                   |
| `GOEXPERIMENT=jsonv2 go vet ./...`                                | Core static analysis                       |
| `golangci-lint run ./...`                                         | Lint core (config in `.golangci.yml`)      |
| `cd viz && GOEXPERIMENT=jsonv2 go test ./...`                     | Run viz tests                              |
| `cd viz && golangci-lint run ./...`                               | Lint viz                                   |
| `cd live && GOEXPERIMENT=jsonv2 go test ./...`                    | Run live tests                             |
| `cd live && golangci-lint run ./...`                              | Lint live                                  |
| `go run ./viz/example`                                            | Run the demo pipeline                      |
| `cd live && GOEXPERIMENT=jsonv2 go run ./demo`                    | Run live dashboard demo (:18080)           |
| `nix run .#check`                                                 | All checks: vet+test-race+lint+govulncheck |

A pull request is mergeable only when **all** of the above pass cleanly.

## Testing Patterns

- External test package (`auditlog_test`) — tests exercise only the public API.
- Standard `testing` + table-driven tests. **No** testify/ginkgo.
- Test step types implement `String()` for deterministic names.
- Retry tests must create a fresh `backoff.NewExponentialBackOff()` to avoid a
  known data race in go-workflow's shared `DefaultRetryOption.Backoff`.
- Aim to keep coverage ≥ 90% of the `auditlog` package.

## Code Style

- Match the existing style: early returns, small focused functions, no
  one-letter names outside tight loops.
- JSON tags use `snake_case` (enforced by `tagliatelle` in `.golangci.yml`).
- Exported identifiers must have doc comments.
- Strong types over runtime checks — prefer typed enums over magic strings.

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add RunID for cross-system correlation
fix: prevent status drift from going undetected
docs: document known limitations
test: add RunID round-trip coverage
chore: bump golangci-lint to v2.4.0
```

Keep the subject line under 72 characters. Explain the **why** in the body when
non-obvious.

## Release Process

Releases follow semantic versioning (`vMAJOR.MINOR.PATCH`). While pre-1.0
(`v0.x`), breaking changes are permitted in minor bumps but should be called out
in the changelog.

1. **Verify CI is green** on `master` (test, lint, coverage, mod-tidy).
2. **Update `CHANGELOG.md`** — move `[Unreleased]` entries under a new version
   heading with the release date.
3. **Bump `SchemaVersion`** in `types.go` if the JSON report schema changed in a
   backwards-incompatible way. The schema version is independent of the module
   tag.
4. **Tag** the commit with three annotated tags (one per module):
   `git tag -a v0.X.Y -m "..."`, `git tag -a viz/v0.X.Y -m "..."`,
   `git tag -a live/v0.X.Y -m "..."`.
5. **Push** the tags: `git push origin v0.X.Y viz/v0.X.Y live/v0.X.Y`.
6. **Create a GitHub Release** from the core tag, pasting the changelog section as the
   release notes. See [`RELEASE.md`](RELEASE.md) for the full process.

`goreleaser` configuration (`.goreleaser.yml`) is provided to automate
release artifacts and changelog generation. To test it locally without
publishing:

```bash
goreleaser release --snapshot --clean
goreleaser check   # validate config
```

This builds the demo binary in `dist/` and renders the changelog without
creating a tag or pushing anything.

## Reporting Issues

Please use [GitHub Issues](https://github.com/LarsArtmann/go-workflow-auditlog/issues)
to report bugs or request features. Include the Go version, go-workflow version,
and a minimal reproduction.
