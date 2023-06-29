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
        pulumi-cli = pkgs.buildGoModule {
          name = "pulumi-cli";
          src = ./pkg;
          subPackages = [ "cmd/pulumi" ];
          vendorSha256 = "sha256-rlxQtxfJ8/0j6PznvIly5x/72zyf3QhMsymrHggw+UU=";
          buildInputs = [ pulumi-sdk ];

          ldflags = let
            version = pkgs.lib.fileContents ./.version;
          in ["-X" "github.com/pulumi/pulumi/pkg/v3/version.Version=${version}"];

          # don't run tests because they make network requests
          doCheck = false; # TODO: fix

          # Add the 'replace' directive to point ot the local pulumi-sdk.
          # Add it as a patch step rather than a build step for better caching.
          prePatch = ''
            go mod edit -replace github.com/pulumi/pulumi/sdk/v3=${pulumi-sdk.src}
          '';
        };
        pulumi-sdk = pkgs.buildGoModule {
          name = "pulumi-sdk";
          src = ./sdk;
          subPackages = [ "go/pulumi" ];
          vendorSha256 = "sha256-S8eb2V7ZHhQ0xas+88lwxjL50+22dbyJ0aM60dAtb5k=";
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
