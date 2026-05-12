{
  description = "Pulumi development environment and CI tools";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    # Older nixpkgs for Python 3.10 (EOL in unstable)
    nixpkgs-python310.url = "github:NixOS/nixpkgs/nixos-24.11";
  };

  outputs = { self, nixpkgs, nixpkgs-python310 }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
          pkgs-old = import nixpkgs-python310 { inherit system; };

          # Pin protoc to exact version — even minor differences change generated output
          protocVersion = "29.5";
          protocPlatform = {
            "x86_64-linux" = { name = "linux-x86_64"; hash = "sha256-o/CUNjzSBcb3rw0bkwXLTIUXBD8mXNsYjwmMrpPoshc="; };
            "aarch64-linux" = { name = "linux-aarch_64"; hash = "sha256-JesISP8TqQoLLSo7Sp0rq8f7/hWPWWwA2MJqIQKN1vU="; };
            "x86_64-darwin" = { name = "osx-x86_64"; hash = "sha256-hPBaI8jV38bMYTiB8pQXQhRz/0nN20yB6p+5SGTfCfs="; };
            "aarch64-darwin" = { name = "osx-aarch_64"; hash = "sha256-qkdxVsT+chF6W4Fhj+CcEvL/GcxYqZyFx7Vpar/qZqQ="; };
          }.${system};

          protoc-pinned = pkgs.stdenv.mkDerivation {
            pname = "protoc";
            version = protocVersion;
            src = pkgs.fetchurl {
              url = "https://github.com/protocolbuffers/protobuf/releases/download/v${protocVersion}/protoc-${protocVersion}-${protocPlatform.name}.zip";
              hash = protocPlatform.hash;
            };
            nativeBuildInputs = [ pkgs.unzip ];
            sourceRoot = ".";
            installPhase = ''
              mkdir -p $out/bin $out/include
              cp bin/protoc $out/bin/
              cp -r include/* $out/include/
            '';
          };

          protoc-gen-go-pinned = pkgs.buildGoModule rec {
            pname = "protoc-gen-go";
            version = "1.36.6";
            src = pkgs.fetchFromGitHub {
              owner = "protocolbuffers";
              repo = "protobuf-go";
              rev = "v${version}";
              hash = "sha256-6Wx1XoHZS1RM0hpgVE85U7huVS4IK+AroTE2zpZR4VI=";
            };
            vendorHash = "sha256-nGI/Bd6eMEoY0sBwWEtyhFowHVvwLKjbT4yfzFz6Z3E=";
            subPackages = [ "cmd/protoc-gen-go" ];
          };

          protoc-gen-go-grpc-pinned = pkgs.buildGoModule rec {
            pname = "protoc-gen-go-grpc";
            version = "1.5.1";
            src = pkgs.fetchFromGitHub {
              owner = "grpc";
              repo = "grpc-go";
              rev = "cmd/protoc-gen-go-grpc/v${version}";
              hash = "sha256-PAUM0chkZCb4hGDQtCgHF3omPm0jP1sSDolx4EuOwXo=";
            };
            vendorHash = "sha256-yn6jo6Ku/bnbSX8FL0B/Uu3Knn59r1arjhsVUkZ0m9g=";
            sourceRoot = "${src.name}/cmd/protoc-gen-go-grpc";
          };

          # JDK11 has bin -> lib/openjdk/bin (a directory symlink) which
          # symlinkJoin doesn't flatten. Create a package with a real bin/.
          jdk11-flat = pkgs.runCommand "jdk11-bin" {} ''
            mkdir -p $out/bin
            for f in ${pkgs.jdk11}/bin/*; do
              ln -s "$f" "$out/bin/"
            done
          '';

          # Common tools shared across all version sets
          commonTools = [
            protoc-pinned
            protoc-gen-go-pinned
            protoc-gen-go-grpc-pinned
            pkgs.jq
            pkgs.uv
            pkgs.bun
            jdk11-flat
            pkgs.gradle
            pkgs.gotestsum
            pkgs.yarn
            pkgs.pnpm_10
            pkgs.poetry
            pkgs.delve
            pkgs.gh
            pkgs.wabt
          ];

          # Build a ci-tools package for a given version set
          mkCiTools = { name, go, python, nodejs, dotnet }: pkgs.symlinkJoin {
            name = "pulumi-ci-tools-${name}";
            paths = commonTools ++ [ go python nodejs dotnet ];
          };
        in
        {
          ci-proto-tools = pkgs.symlinkJoin {
            name = "pulumi-ci-proto-tools";
            paths = [
              protoc-pinned
              protoc-gen-go-pinned
              protoc-gen-go-grpc-pinned
              pkgs.jq
              pkgs.uv
              pkgs.python3
            ];
          };

          ci-tools-current = mkCiTools {
            name = "current";
            go = pkgs.go;              # 1.26.x
            python = pkgs.python314;
            nodejs = pkgs.nodejs_25;
            dotnet = pkgs.dotnet-sdk_9;
          };

          ci-tools-minimum = mkCiTools {
            name = "minimum";
            go = pkgs.go_1_25;
            python = pkgs-old.python310;
            nodejs = pkgs.nodejs_20;
            dotnet = pkgs.dotnet-sdk_8;
          };
        }
      );

      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            buildInputs = [
              self.packages.${system}.ci-tools-current
              pkgs.gopls
            ];
          };
        }
      );
    };
}
