"""Rule for creating Python wheels with version stamping.

Works around py_wheel's analysis-time version validation by building with
a placeholder version, then patching to the real version at build time.
This allows `bazel test //...` to work without --define PYPI_VERSION.
"""

def _stamped_wheel_impl(ctx):
    version = ctx.var.get("PYPI_VERSION", ctx.attr.version_placeholder)
    placeholder = ctx.attr.version_placeholder

    out = ctx.actions.declare_file(ctx.attr.name + ".whl")
    staging = ctx.actions.declare_directory(ctx.attr.name + "_staging")

    # Collect type stub files and their destination paths within the wheel.
    pyi_inputs = []
    pyi_commands = []
    for f in ctx.files.type_stubs:
        pyi_inputs.append(f)
        path = f.short_path
        # Strip sdk/python/lib/ prefix to get wheel-internal path (e.g. pulumi/foo.pyi).
        idx = path.find("lib/")
        if idx >= 0:
            dest = path[idx + 4:]
        else:
            dest = f.basename
        pyi_commands.append(
            "mkdir -p \"$S/$(dirname '%s')\" && cp '%s' \"$S/%s\"" % (dest, f.path, dest),
        )

    base_wheel = ctx.file.wheel

    cmd = """\
set -euo pipefail
S="{staging}"
OUTPATH="$(pwd)/{out}"
mkdir -p "$S"

# Extract the base wheel.
unzip -qo "{wheel}" -d "$S"

# Patch version if different from placeholder.
if [ "{placeholder}" != "{version}" ]; then
    OLD_DIST="$S/pulumi-{placeholder}.dist-info"
    NEW_DIST="$S/pulumi-{version}.dist-info"
    if [ -d "$OLD_DIST" ]; then
        mv "$OLD_DIST" "$NEW_DIST"
    fi
    # Patch METADATA.
    if [ -f "$NEW_DIST/METADATA" ]; then
        perl -pi -e 's/^Version: \\Q{placeholder}\\E$/Version: {version}/' "$NEW_DIST/METADATA"
    fi
    # Patch RECORD paths.
    if [ -f "$NEW_DIST/RECORD" ]; then
        perl -pi -e 's/pulumi-\\Q{placeholder}\\E\\.dist-info/pulumi-{version}.dist-info/g' "$NEW_DIST/RECORD"
    fi
fi

# Add .pyi type stubs and py.typed marker.
{pyi_commands}

# Repack as wheel.
cd "$S"
zip -qr "$OUTPATH" .
""".format(
        staging = staging.path,
        wheel = base_wheel.path,
        placeholder = placeholder,
        version = version,
        pyi_commands = "\n".join(pyi_commands) if pyi_commands else "true",
        out = out.path,
    )

    ctx.actions.run_shell(
        inputs = [base_wheel] + pyi_inputs,
        outputs = [staging, out],
        command = cmd,
        mnemonic = "StampWheel",
        progress_message = "Stamping Python wheel %s" % ctx.label,
    )

    return [DefaultInfo(files = depset([out]))]

stamped_wheel = rule(
    implementation = _stamped_wheel_impl,
    attrs = {
        "wheel": attr.label(
            mandatory = True,
            allow_single_file = [".whl"],
            doc = "The base py_wheel target to stamp.",
        ),
        "type_stubs": attr.label_list(
            allow_files = True,
            doc = ".pyi type stubs and py.typed markers to inject into the wheel.",
        ),
        "version_placeholder": attr.string(
            default = "0.0.0",
            doc = "The placeholder version used by the base wheel.",
        ),
    },
    doc = "Stamp a py_wheel with version from --define PYPI_VERSION and inject type stubs.",
)
