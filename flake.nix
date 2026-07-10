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
                ;
            };

            BUILDFLOW_LANGUAGE = "go";
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

          treefmt = {
            programs = {
              nixfmt.enable = true;
              gofmt.enable = true;
            };
          };

          checks.format = config.treefmt.build.check self;
        };
    };
}
