{
  pkgs ? import ./nixpkgs.nix {}
}:
  pkgs.mkShell {
    LD_LIBRARY_PATH = pkgs.lib.makeLibraryPath [
      pkgs.stdenv.cc.cc
    ];

    LOCALE_ARCHIVE = "${pkgs.glibcLocales}/lib/locale/locale-archive";

    buildInputs = [
      pkgs.bazel_7
      pkgs.dotnet-sdk
      pkgs.git
      pkgs.glibcLocales
      pkgs.go_1_22
      pkgs.gofumpt
      pkgs.golangci-lint
      pkgs.gopls
      pkgs.jq
      pkgs.nix
      pkgs.nodePackages.pnpm
      pkgs.nodePackages.yarn
      pkgs.nodejs-18_x
      pkgs.pulumictl
      pkgs.python3
    ];

    shellHook = ''
      export PULUMI_ROOT=$(pwd)/.root
      export PATH=$PULUMI_ROOT/bin:$PATH
    '';
  }
