{
  description = "Pulumi development environment and CI tools";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };

          # Pin protoc to the exact version used for code generation.
          # We download the pre-built binary to match the version exactly,
          # since even minor version differences (29.5 vs 29.6) change
          # generated output and fail `make check_proto`.
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

          # Pin protoc-gen-go to the exact version used for code generation
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

          # Pin protoc-gen-go-grpc to the exact version used for code generation
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
        in
        {
          # CI proto toolchain — used by GitHub Actions composite action
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
        }
      );

      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              # Proto toolchain
              self.packages.${system}.ci-proto-tools

              # Core languages (default versions for local dev)
              go
              nodejs_20
              python311
              dotnet-sdk_8

              # Go tools
              delve
              gopls
              gofumpt
              golangci-lint

              # Build tools
              jq
              gh
              bun
              uv
              wabt

              # Java (for Gradle-based tests)
              jdk11
              gradle
            ];
          };
        }
      );
    };
}
