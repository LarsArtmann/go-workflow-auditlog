# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks, verified against the actual code.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md) — never retained here.

---

## Blocked (external dependency)

- [ ] **Bump Go toolchain 1.26.4 → 1.26.5** to resolve GO-2026-5856 (crypto/tls
  Encrypted-Client-Hello privacy leak). `live.Server.ListenAndServe` is an
  affected call path — confirmed via `govulncheck` ("Fixed in: crypto/tls@go1.26.5").
  **Why blocked:** the pinned `nixos-unstable` revision (`2cc9de6`) ships
  `go_1_26` at **1.26.4**. The `go` directive is a *minimum*, so bumping
  `go.mod` / `go.work` to `1.26.5` while the installed toolchain is 1.26.4
  breaks the build. **Remediation** (once nixpkgs provides `go_1_26` ≥ 1.26.5):
  bump `go 1.26.4` → `1.26.5` in `go.mod`, `viz/go.mod`, `live/go.mod`,
  `go.work`, and `.golangci.yml` (`run.go`); then re-enable the deferred
  `govulncheck` block for live in `flake.nix`.
- [ ] **Add `govulncheck` for the `live` module to `nix run .#check`** — live
  now runs `go vet` + `go test -race` + `golangci-lint` in CI, but `govulncheck`
  is intentionally omitted: it reports GO-2026-5856 (exits code 3) and would
  keep `nix run .#check` permanently red until the toolchain bump above lands.
  The exact re-enable command is documented inline in `flake.nix`. Unblocks
  automatically with the Go bump.

## Deferred (design decision)

- [ ] **Full browser automation tests (Playwright or chromedp)** for the live
  dashboard — **deferred by design**. `go-workflow-auditlog` is a Go *library*,
  not an application. Adding a browser-binary runtime dependency (Chromium,
  ~300 MB) to the default `go test` / `nix run .#check` would make CI heavy,
  slow, platform-dependent, and flaky — a net negative for a library's CI. The
  existing Go-based JS structural tests already cover function presence, event
  wiring, diff-based rendering infrastructure, WebSocket fallback, and CSS
  integrity. If pixel-level click→render→DOM flows become necessary, gate them
  behind a `//go:build browser_e2e` tag (excluded from default CI) and run them
  only in a dedicated environment with Chromium provisioned. Tracked as a
  candidate in [ROADMAP.md](./ROADMAP.md).
