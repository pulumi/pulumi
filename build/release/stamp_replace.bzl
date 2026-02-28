"""Rules for replacing placeholder versions with stamped values."""

def _version_replace_impl(ctx):
    version = ctx.var.get("PULUMI_VERSION", "0.0.0-dev")
    placeholder = ctx.attr.placeholder
    out = ctx.actions.declare_file(ctx.attr.out)
    ctx.actions.expand_template(
        template = ctx.file.src,
        output = out,
        substitutions = {
            placeholder: version,
        },
    )
    return [DefaultInfo(files = depset([out]))]

version_replace = rule(
    implementation = _version_replace_impl,
    attrs = {
        "src": attr.label(
            mandatory = True,
            allow_single_file = True,
            doc = "Source file containing the placeholder.",
        ),
        "out": attr.string(
            mandatory = True,
            doc = "Output file name.",
        ),
        "placeholder": attr.string(
            mandatory = True,
            doc = "The placeholder string to replace with the version.",
        ),
    },
    doc = "Replace a placeholder string in a file with PULUMI_VERSION from --define.",
)
