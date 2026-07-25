# TODO List — go-workflow-auditlog

Actionable short- and mid-term tasks, verified against the actual code.
Long-term vision and raw ideas live in [ROADMAP.md](./ROADMAP.md).
Completed items are documented in [CHANGELOG.md](./CHANGELOG.md) — never retained here.

---

## High Impact

- [ ] **Add the `live` module to `nix run .#check`** — the canonical check script (`flake.nix`) runs vet + test-race + lint + govulncheck for core and viz, but NOT live. The module that depends on go-sse + gorilla/websocket has zero CI coverage. Evidence: `flake.nix` `check` package only loops core + viz.
- [ ] **Bump Go toolchain 1.26.4 → 1.26.5** to resolve GO-2026-5856 (crypto/tls ECH privacy leak). `live.Server.ListenAndServe` is an affected call path. Requires a nixpkgs revision providing `go_1_26` at 1.26.5. Evidence: `go.mod` pins `go 1.26.4`; `flake.nix` uses `pkgs.go_1_26`.
- [ ] **Improve live module test coverage to 95%+** (currently 90.3%). Gap is concentrated in error/timing paths: `handleSSE` heartbeat/write-failure/cancellation, `sendSnapshot`/`sendComplete` build errors, `handleWebSocket` upgrade failure, `writeWS` deadline/marshal failure. Achievable via interface extraction for mockable dependencies.

## Medium Impact

- [ ] **Add CSV escaping / formula-injection tests** — `csv.go` has no test for step names containing commas, quotes, newlines, or formula vectors (`=cmd`, `+cmd`, `-cmd`, `@cmd`). `encoding/csv` handles quoting, but a dedicated regression test should lock it in. Evidence: `csv_test.go` has no injection/escaping case.
- [ ] **Add full browser automation tests (Playwright or chromedp)** for the live dashboard — current Go-based JS structural tests validate function presence, event wiring, diff-based rendering infrastructure, WebSocket fallback, and CSS integrity, but cannot test click→render→DOM-update flows at the pixel level (e.g. graph node click → Steps tab opens + row highlights).
- [ ] **Update `docs/DOMAIN_LANGUAGE.md`** with missing live-module terms: WebSocket transport (`/api/ws`, SSE→WS fallback), `CaptureDAG(w)`, `CORSAllowedOrigins` (secure-by-default), configurable `Prefix`, export endpoints/buttons. The file already covers Hub/SSE/snapshot/event/complete but not these.
- [ ] **Resolve the `nlreturn` vs `wsl_v5` linter conflict** in `.golangci.yml` — both linters are enabled and directly contradict each other on blank-line-before-return, requiring `//nolint` workarounds. Either disable one or add a project-level exception for the conflicting pattern.
