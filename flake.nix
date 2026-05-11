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
              pkgs.protobuf_29
              protoc-gen-go-pinned
              protoc-gen-go-grpc-pinned
              pkgs.jq
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
              protobuf_29
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
