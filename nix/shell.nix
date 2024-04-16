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
      pkgs.buildifier
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
      pkgs.nodePackages.typescript-language-server
      pkgs.nodejs-18_x
      pkgs.pulumictl
      pkgs.python3
    ];

    shellHook = ''
    '';
  }
