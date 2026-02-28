"""Rule for creating npm tarballs with version stamping."""

def _npm_tarball_impl(ctx):
    version = ctx.var.get("PULUMI_VERSION", "0.0.0-dev")
    placeholder = ctx.attr.version_placeholder

    out = ctx.actions.declare_file(ctx.attr.name + ".tgz")
    staging = ctx.actions.declare_directory(ctx.attr.name + "_staging")

    # Collect all inputs.
    inputs = []
    copy_commands = []

    # 1. Compiled JS files from ts_project (in bin/ subdir).
    #    Skip package.json from compiled output - we'll use the source version.
    for f in ctx.files.compiled_js:
        path = f.short_path
        idx = path.find("/bin/")
        if idx >= 0:
            dest = path[idx + 5:]
        else:
            dest = f.basename

        if dest == "package.json":
            continue

        inputs.append(f)
        copy_commands.append("mkdir -p \"$S/$(dirname '%s')\" && cp '%s' \"$S/%s\"" % (dest, f.path, dest))

    # 2. Proto source files.
    for f in ctx.files.proto_srcs:
        path = f.short_path
        idx = path.find("proto/")
        if idx >= 0:
            dest = path[idx:]
        else:
            dest = "proto/" + f.basename
        inputs.append(f)
        copy_commands.append("mkdir -p \"$S/$(dirname '%s')\" && cp '%s' \"$S/%s\"" % (dest, f.path, dest))

    # 3. Vendor files.
    for f in ctx.files.vendor:
        path = f.short_path
        idx = path.find("vendor/")
        if idx >= 0:
            dest = path[idx:]
        else:
            dest = "vendor/" + f.basename
        inputs.append(f)
        copy_commands.append("mkdir -p \"$S/$(dirname '%s')\" && cp '%s' \"$S/%s\"" % (dest, f.path, dest))

    # 4. Metadata / dist files - placed at package root.
    for f in ctx.files.package_json + ctx.files.extra_srcs:
        inputs.append(f)
        copy_commands.append("cp -f '%s' \"$S/%s\"" % (f.path, f.basename))

    # Build the command: assemble, patch version, create tarball.
    cmd = """\
set -euo pipefail
S="{staging}"
mkdir -p "$S"

{copy_commands}

# Make files writable for version patching.
chmod -R u+w "$S"

# Patch version placeholder with actual version.
if [ "{version}" != "{placeholder}" ]; then
    perl -pi -e 's/\\Q{placeholder}\\E/{version}/g' "$S/package.json" "$S/version.js" 2>/dev/null || true
fi

# Create npm tarball (npm tarballs are gzip'd tars with a package/ prefix).
# Use a wrapper directory to get the package/ prefix without --transform (macOS compat).
WRAPPER=$(dirname "$S")/_tarwrap
mkdir -p "$WRAPPER"
ln -s "$(cd "$S" && pwd)" "$WRAPPER/package"
tar czfh "{out}" -C "$WRAPPER" package
""".format(
        staging = staging.path,
        copy_commands = "\n".join(copy_commands),
        version = version,
        placeholder = placeholder,
        out = out.path,
    )

    ctx.actions.run_shell(
        inputs = inputs,
        outputs = [staging, out],
        command = cmd,
        mnemonic = "NpmPack",
        progress_message = "Packing npm tarball %s" % ctx.label,
    )

    return [DefaultInfo(files = depset([out]))]

npm_tarball = rule(
    implementation = _npm_tarball_impl,
    attrs = {
        "compiled_js": attr.label(
            mandatory = True,
            doc = "The ts_project target producing compiled JS.",
        ),
        "proto_srcs": attr.label_list(
            allow_files = True,
            doc = "Proto-generated JS/TS source files.",
        ),
        "vendor": attr.label_list(
            allow_files = True,
            doc = "Vendored dependency files.",
        ),
        "package_json": attr.label_list(
            allow_files = True,
            mandatory = True,
            doc = "package.json file.",
        ),
        "extra_srcs": attr.label_list(
            allow_files = True,
            doc = "Additional files to include at the package root (.npmignore, README, LICENSE, dist scripts).",
        ),
        "version_placeholder": attr.string(
            default = "0.0.0",
            doc = "The placeholder version string to replace.",
        ),
    },
    doc = "Create an npm tarball (.tgz) with version stamping.",
)
