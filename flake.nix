{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  # NOTE:
  # The SHA256 hashes below have to be updated whenever dependencies change.
  # For a longer explanation, see the comments in:
  # https://github.com/tailscale/tailscale/blob/1ca5dcce15fefd218bb297656c43a65df4e410b5/flake.nix
  #
  # We may be able to automate this in the future with something like:
  # https://github.com/tailscale/tailscale/blob/1ca5dcce15fefd218bb297656c43a65df4e410b5/update-flake.sh

  outputs = { self, nixpkgs, flake-utils }: let
    pulumi-packages = pkgs: rec {
        pulumi-version = pkgs.lib.fileContents ./.version;
        pulumi-sdk = import ./sdk/module.nix {
          nixpkgs = pkgs;
        };
        pulumi-cli = import ./pkg/module.nix {
          nixpkgs = pkgs;
          pulumi-sdk = pulumi-sdk;
          pulumi-version = pulumi-version;
        };
      };

    flakeForSystem = nixpkgs: system: let
      pkgs = nixpkgs.legacyPackages.${system};
      pulumi = pulumi-packages pkgs;
      in {
        packages = pulumi;
        devShell = pkgs.mkShell {
          packages = with pkgs; [ git gopls go golangci-lint ];
        };
      };
    in flake-utils.lib.eachDefaultSystem (system: flakeForSystem nixpkgs system);
  }
