"""Macros for building cross-compiled Pulumi release binaries."""

load("@rules_go//go:def.bzl", "go_binary")

# All platforms we release for.
PLATFORMS = [
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("darwin", "amd64"),
    ("darwin", "arm64"),
    ("windows", "amd64"),
    ("windows", "arm64"),
]

def pulumi_release_binary(name, embed, gotags = [], x_defs = {}, visibility = None):
    """Generate a cross-compiled go_binary for each release platform.

    Creates targets named {name}_{goos}_{goarch} for each platform in PLATFORMS.

    Args:
        name: Base name for the generated targets.
        embed: The go_library to embed (e.g. ["//pkg/cmd/pulumi:pulumi_lib"]).
        gotags: Go build tags to apply.
        x_defs: x_defs dict for version stamping.
        visibility: Visibility for the generated targets.
    """
    for goos, goarch in PLATFORMS:
        go_binary(
            name = "%s_%s_%s" % (name, goos, goarch),
            embed = embed,
            goos = goos,
            goarch = goarch,
            gotags = gotags,
            gc_linkopts = ["-w", "-s"],
            x_defs = x_defs,
            visibility = visibility or ["//visibility:public"],
        )
