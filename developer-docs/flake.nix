# To build and peruse the HTML version of the developer-docs:
#
#     nix build
#     open result/index.html
{
  inputs = {
    nixpkgs.url = github:NixOS/nixpkgs/nixos-24.05;
  };

  outputs = { self, nixpkgs }: let

    packagePulumiDeveloperDocs = sys: let
      version = "0.0.1";
      pkgs = import nixpkgs { system = sys; };
    in pkgs.stdenv.mkDerivation {
      name = "pulumi-developer-docs-${version}";
      nativeBuildInputs = [
        pkgs.sphinx
        pkgs.python311Packages.myst-parser
        pkgs.python311Packages.sphinx-rtd-theme
        pkgs.python311Packages.sphinx-tabs
      ];
      version = "${version}";
      src = ../.;
      buildPhase = "cd developer-docs && make";
      installPhase = "mv _build/html $out";
    };

    packages = sys: let
      pkgs = import nixpkgs { system = sys; };
    in {
      default = packagePulumiDeveloperDocs sys;
    };

  in {
    packages.x86_64-linux = packages "x86_64-linux";
    packages.x86_64-darwin = packages "x86_64-darwin";
    packages.aarch64-darwin = packages "aarch64-darwin";
    packages.aarch64-linux = packages "aarch64-linux";
  };
}
