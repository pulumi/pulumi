"""Macros for creating Pulumi release archives."""

load("@rules_pkg//pkg:tar.bzl", "pkg_tar")
load("@rules_pkg//pkg:zip.bzl", "pkg_zip")
load("@rules_pkg//pkg:mappings.bzl", "pkg_files", "pkg_attributes")

# The four Go binaries built from this repo.
_GO_BINARIES = [
    "pulumi",
    "pulumi-language-go",
    "pulumi-language-nodejs",
    "pulumi-language-python",
]

# External language providers to include.
_EXTERNAL_LANGS = ["dotnet", "java", "yaml"]

def _archive_arch(goarch):
    """Map Go arch names to archive arch names (amd64 -> x64)."""
    if goarch == "amd64":
        return "x64"
    return goarch

def pulumi_release_archives(name):
    """Generate release archives for all platforms.

    Creates targets named archive_{goos}_{goarch} for each platform,
    plus an all_archives filegroup.

    Args:
        name: Unused (required by Bazel macro convention).
    """
    archive_targets = []

    for goos, goarch in [
        ("linux", "amd64"),
        ("linux", "arm64"),
        ("darwin", "amd64"),
        ("darwin", "arm64"),
        ("windows", "amd64"),
        ("windows", "arm64"),
    ]:
        _pulumi_release_archive(goos, goarch)
        archive_targets.append(":archive_%s_%s" % (goos, goarch))

    native.filegroup(
        name = "all_archives",
        srcs = archive_targets,
        visibility = ["//visibility:public"],
    )

def _pulumi_release_archive(goos, goarch):
    """Create a release archive for a single platform."""
    arch_label = _archive_arch(goarch)
    ext = ".exe" if goos == "windows" else ""

    # Prefix inside the archive: "pulumi" for unix, "pulumi/bin" for windows.
    prefix = "pulumi/bin" if goos == "windows" else "pulumi"

    # ── Collect Go binaries built from this repo ──
    # Rename from Bazel target names (e.g. pulumi_linux_amd64) to release
    # names (e.g. pulumi).
    go_binary_renames = {}
    go_binary_srcs = []
    for binary in _GO_BINARIES:
        label = ":%s_%s_%s" % (binary, goos, goarch)
        go_binary_srcs.append(label)
        go_binary_renames[label] = "%s/%s%s" % (prefix, binary, ext)

    pkg_files(
        name = "_files_go_%s_%s" % (goos, goarch),
        srcs = go_binary_srcs,
        renames = go_binary_renames,
        attributes = pkg_attributes(mode = "0755"),
    )

    # ── Collect OS-specific scripts ──
    if goos == "windows":
        script_srcs = [
            "//sdk/nodejs:dist/pulumi-resource-pulumi-nodejs.cmd",
            "//sdk/python:dist/pulumi-resource-pulumi-python.cmd",
            "//sdk/python:cmd/pulumi-language-python-exec",
        ]
    else:
        script_srcs = [
            "//sdk/nodejs:dist/pulumi-resource-pulumi-nodejs",
            "//sdk/python:dist/pulumi-resource-pulumi-python",
            "//sdk/python:cmd/pulumi-language-python-exec",
        ]

    pkg_files(
        name = "_files_scripts_%s_%s" % (goos, goarch),
        srcs = script_srcs,
        prefix = prefix,
        attributes = pkg_attributes(mode = "0755"),
    )

    # ── Collect external language providers ──
    lang_srcs = []
    for lang in _EXTERNAL_LANGS:
        lang_srcs.append(
            "@pulumi_language_%s_%s_%s//:pulumi-language-%s%s" % (lang, goos, goarch, lang, ext),
        )

    # ── Collect pulumi-watch (absent for windows-arm64) ──
    watch_key = "%s_%s" % (goos, goarch)
    watch_srcs = []
    if watch_key != "windows_arm64":
        watch_srcs.append(
            "@pulumi_watch_%s//:pulumi-watch%s" % (watch_key, ext),
        )

    external_srcs = lang_srcs + watch_srcs

    pkg_files(
        name = "_files_external_%s_%s" % (goos, goarch),
        srcs = external_srcs,
        prefix = prefix,
        attributes = pkg_attributes(mode = "0755"),
    )

    # ── Create archive ──
    all_file_groups = [
        ":_files_go_%s_%s" % (goos, goarch),
        ":_files_scripts_%s_%s" % (goos, goarch),
        ":_files_external_%s_%s" % (goos, goarch),
    ]

    if goos == "windows":
        pkg_zip(
            name = "archive_%s_%s" % (goos, goarch),
            srcs = all_file_groups,
            package_file_name = "pulumi-v{PULUMI_VERSION}-%s-%s.zip" % (goos, arch_label),
            stamp = 1,
            visibility = ["//visibility:public"],
        )
    else:
        pkg_tar(
            name = "archive_%s_%s" % (goos, goarch),
            srcs = all_file_groups,
            extension = "tar.gz",
            package_file_name = "pulumi-v{PULUMI_VERSION}-%s-%s.tar.gz" % (goos, arch_label),
            stamp = 1,
            visibility = ["//visibility:public"],
        )
