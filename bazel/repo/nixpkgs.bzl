load(
    "@io_tweag_rules_nixpkgs//nixpkgs:nixpkgs.bzl",
    "nixpkgs_local_repository",
)

def setup_nixpkgs(name = "setup_nixpkgs"):
    nixpkgs_local_repository(
        name = "nixpkgs",
        nix_file = "//nix:nixpkgs.nix",
    )
