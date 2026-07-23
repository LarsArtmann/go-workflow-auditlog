{
  description = "Audit logging library for Azure/go-workflow";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    systems.url = "github:nix-systems/default";

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs@{ self, flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = import inputs.systems;

      imports = [ inputs.treefmt-nix.flakeModule ];

      perSystem =
        {
          config,
          pkgs,
          lib,
          ...
        }:
        {
          devShells.default = pkgs.mkShellNoCC {
            packages = builtins.attrValues {
              inherit (pkgs)
                go_1_26
                golangci-lint
                actionlint
                govulncheck
                golines
                d2
                ;
            };

            BUILDFLOW_LANGUAGE = "go";
            GOEXPERIMENT = "jsonv2";
          };

          packages.default =
            pkgs.runCommand "go-workflow-auditlog"
              {
                meta = with lib; {
                  description = "Audit logging library for Azure/go-workflow";
                  homepage = "https://github.com/larsartmann/go-workflow-auditlog";
                  license = licenses.mit;
                  platforms = platforms.unix;
                };
              }
              ''
                mkdir -p $out
              '';

          packages.check = pkgs.writeShellApplication {
            name = "check";

            runtimeInputs = [
              pkgs.go_1_26
              pkgs.golangci-lint
              pkgs.govulncheck
            ];

            text = ''
              export GOEXPERIMENT=jsonv2

              echo "==> go vet (core)"
              go vet ./...

              echo "==> go test -race (core)"
              go test -race -count=1 ./...

              echo "==> golangci-lint (core)"
              golangci-lint run --timeout=10m ./...

              echo "==> govulncheck (core)"
              govulncheck ./...

              echo "==> go vet (viz standalone)"
              (cd viz && go vet ./...)

              echo "==> go test -race (viz standalone)"
              (cd viz && GOWORK=off go test -race -count=1 ./...)

              echo "==> golangci-lint (viz)"
              (cd viz && golangci-lint run --timeout=10m ./...)

              echo "==> govulncheck (viz)"
              (cd viz && GOWORK=off govulncheck ./...)

              echo "All checks passed."
            '';
          };

          treefmt = {
            programs = {
              nixfmt.enable = true;
              gofmt.enable = true;
            };

            settings.formatter.d2 = {
              command = "${pkgs.d2}/bin/d2";
              options = [ "fmt" ];
              includes = [ "*.d2" ];
            };
          };

          checks.build = config.packages.default;
          checks.format = config.treefmt.build.check self;
        };
    };
}
