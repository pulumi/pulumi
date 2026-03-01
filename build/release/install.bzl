"""Macro for creating a local install directory of Pulumi binaries.

Uses select() on platform constraints to automatically pick the right
cross-compiled binaries for the host platform. The result is a flat directory
containing all Pulumi binaries, equivalent to ~/.pulumi-dev/bin from make install.

Usage:
    bazel build //build/release:install
"""

# Platform constraint mappings: (goos, goarch, os_constraint, cpu_constraint).
_PLATFORMS = [
    ("linux", "amd64", "@platforms//os:linux", "@platforms//cpu:x86_64"),
    ("linux", "arm64", "@platforms//os:linux", "@platforms//cpu:aarch64"),
    ("darwin", "amd64", "@platforms//os:macos", "@platforms//cpu:x86_64"),
    ("darwin", "arm64", "@platforms//os:macos", "@platforms//cpu:aarch64"),
    ("windows", "amd64", "@platforms//os:windows", "@platforms//cpu:x86_64"),
    ("windows", "arm64", "@platforms//os:windows", "@platforms//cpu:aarch64"),
]

_GO_BINARIES = ["pulumi", "pulumi-language-go", "pulumi-language-nodejs", "pulumi-language-python"]
_EXTERNAL_LANGS = ["dotnet", "java", "yaml"]
_PLATFORM_SUFFIXES = ["%s_%s" % (p[0], p[1]) for p in _PLATFORMS]

def _install_name(filename):
    """Map a Bazel output filename to its installed name.

    Go binaries have platform suffixes (e.g. pulumi_darwin_arm64) that need
    to be stripped. All other files keep their original names.
    """
    base = filename
    exe = ""
    if base.endswith(".exe"):
        base = base[:-4]
        exe = ".exe"

    for binary in _GO_BINARIES:
        for suffix in _PLATFORM_SUFFIXES:
            if base == binary + "_" + suffix:
                return binary + exe

    return filename

def _pulumi_install_dir_impl(ctx):
    out = ctx.actions.declare_directory(ctx.attr.name)

    inputs = []
    commands = ["mkdir -p \"%s\"" % out.path]

    for src in ctx.attr.srcs:
        for f in src[DefaultInfo].files.to_list():
            inputs.append(f)
            dest_name = _install_name(f.basename)
            commands.append("cp \"%s\" \"%s/%s\"" % (f.path, out.path, dest_name))
            commands.append("chmod 0755 \"%s/%s\"" % (out.path, dest_name))

    ctx.actions.run_shell(
        inputs = inputs,
        outputs = [out],
        command = "\n".join(commands),
        mnemonic = "PulumiInstall",
        progress_message = "Assembling Pulumi install directory %s" % ctx.label,
    )

    return [DefaultInfo(files = depset([out]))]

_pulumi_install_dir = rule(
    implementation = _pulumi_install_dir_impl,
    attrs = {
        "srcs": attr.label_list(
            mandatory = True,
            allow_files = True,
            doc = "All files to include in the install directory.",
        ),
    },
    doc = "Assemble a flat directory of Pulumi binaries for local installation.",
)

def _install_srcs(goos, goarch):
    """Build the list of source labels for a given platform."""
    ext = ".exe" if goos == "windows" else ""

    srcs = []

    # Go binaries built from this repo.
    for binary in _GO_BINARIES:
        srcs.append(":%s_%s_%s" % (binary, goos, goarch))

    # SDK entry-point scripts.
    if goos == "windows":
        srcs.append("//sdk/nodejs:dist/pulumi-resource-pulumi-nodejs.cmd")
        srcs.append("//sdk/python:dist/pulumi-resource-pulumi-python.cmd")
    else:
        srcs.append("//sdk/nodejs:dist/pulumi-resource-pulumi-nodejs")
        srcs.append("//sdk/python:dist/pulumi-resource-pulumi-python")
    srcs.append("//sdk/python:cmd/pulumi-language-python-exec")

    # External language providers.
    for lang in _EXTERNAL_LANGS:
        srcs.append("@pulumi_language_%s_%s_%s//:pulumi-language-%s%s" % (lang, goos, goarch, lang, ext))

    # pulumi-watch (absent for windows-arm64).
    if "%s_%s" % (goos, goarch) != "windows_arm64":
        srcs.append("@pulumi_watch_%s_%s//:pulumi-watch%s" % (goos, goarch, ext))

    return srcs

def pulumi_install(name, **kwargs):
    """Create a local install directory for the host platform.

    Uses select() on platform constraints to pick the correct cross-compiled
    binaries for the machine running the build. The output is a flat directory
    containing all Pulumi binaries and scripts, equivalent to the layout
    produced by make install in ~/.pulumi-dev/bin.

    Build with:
        bazel build //build/release:install

    The resulting directory is at:
        bazel-bin/build/release/install/

    Args:
        name: Target name.
        **kwargs: Additional args passed to the underlying rule.
    """

    # Define config_settings for each platform.
    for goos, goarch, os_constraint, cpu_constraint in _PLATFORMS:
        native.config_setting(
            name = "_%s_is_%s_%s" % (name, goos, goarch),
            constraint_values = [os_constraint, cpu_constraint],
        )

    # Build the select() dict.
    platform_select = {}
    for goos, goarch, _, _ in _PLATFORMS:
        platform_select[":_%s_is_%s_%s" % (name, goos, goarch)] = _install_srcs(goos, goarch)

    _pulumi_install_dir(
        name = name,
        srcs = select(platform_select),
        **kwargs
    )
