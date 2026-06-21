{
  description = "DevShell for go-workflow-auditlog — Go 1.26, golangci-lint, govulncheck";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    { nixpkgs, flake-utils, self, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Core toolchain — pinned to match go.mod (Go 1.26.3).
            go_1_26

            # Linting and analysis.
            golangci-lint

            # GitHub Actions workflow validation.
            actionlint

            # Vulnerability scanning.
            govulncheck

            # Code formatting.
            golines
            nixpkgs-fmt
          ];

          # buildflow auto-detects "nix" when flake.nix is present; force Go.
          BUILDFLOW_LANGUAGE = "go";
        };

        # This is a Go library — there is no buildable binary. The package
        # output provides metadata so `nix build` succeeds for tooling that
        # expects a default derivation.
        packages.default = pkgs.runCommand "go-workflow-auditlog"
          {
            meta = with pkgs.lib; {
              description = "Audit logging library for Azure/go-workflow";
              homepage = "https://github.com/larsartmann/go-workflow-auditlog";
              license = licenses.mit;
              platforms = platforms.unix;
            };
          } ''
          mkdir -p $out
          cat > $out/README <<EOF
          go-workflow-auditlog — audit logging library for Azure/go-workflow.
          This is a library; use devShells.default for development.
          EOF
        '';

        formatter = pkgs.nixpkgs-fmt;
      }
    );
}
