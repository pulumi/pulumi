load("@rules_proto//proto:defs.bzl", "ProtoInfo", "proto_common")

load("@aspect_bazel_lib//lib:copy_to_bin.bzl", "COPY_FILE_TO_BIN_TOOLCHAINS")
load("@aspect_rules_js//js:libs.bzl", "js_lib_helpers")
load("@aspect_rules_js//js:providers.bzl", "JsInfo", "js_info")

def _protoc_action(ctx, proto_info, outputs, options = {
    "keep_empty_files": True,
    "target": "js+dts",
}):
    inputs = depset(
        proto_info.direct_sources,
        transitive = [proto_info.transitive_descriptor_sets]
    )

    args = ctx.actions.args()
    args.add("--js_out=import_style=commonjs,binary:{}".format(ctx.bin_dir.path))
    args.add("--ts_out=grpc_js:{}".format(ctx.bin_dir.path))
    args.add("--grpc_out=grpc_js,minimum_node_version=6:{}".format(ctx.bin_dir.path))

    args.add("--plugin=protoc-gen-js={}".format(ctx.executable._protoc_gen_js.path))
    args.add("--plugin=protoc-gen-grpc={}".format(ctx.executable._grpc_tools_node_protoc.path))
    args.add("--plugin=protoc-gen-ts={}".format(ctx.executable._grpc_tools_node_protoc_ts.path))

    args.add("--descriptor_set_in")
    args.add_joined(proto_info.transitive_descriptor_sets, join_with = ":")

    args.add_all(proto_info.direct_sources)

    ctx.actions.run(
        executable = ctx.executable._protoc,
        progress_message = "Generating .js/.d.ts from %{label}",
        outputs = outputs,
        inputs = inputs,
        mnemonic = "TsProtoLibrary",
        arguments = [args],
        tools = [
            ctx.executable._grpc_tools_node_protoc,
            ctx.executable._grpc_tools_node_protoc_ts,
            ctx.executable._protoc,
            ctx.executable._protoc_gen_js,
        ],
        env = {
            "BAZEL_BINDIR": ctx.bin_dir.path
        },
    )

def _declare_outs(ctx, info, ext):
    pb_outs = proto_common.declare_generated_files(ctx.actions, info, "_pb" + ext)
    grpc_outs = proto_common.declare_generated_files(ctx.actions, info, "_grpc_pb" + ext)
    outs = pb_outs + grpc_outs
    return outs

def _ts_proto_library_impl(ctx):
    info = ctx.attr.proto[ProtoInfo]

    js_outs = _declare_outs(ctx, info, ".js")
    dts_outs = _declare_outs(ctx, info, ".d.ts")

    _protoc_action(ctx, info, js_outs + dts_outs)

    direct_srcs = depset(js_outs)
    direct_decls = depset(dts_outs)

    return [
        DefaultInfo(
            files = direct_srcs,
            runfiles = js_lib_helpers.gather_runfiles(
                ctx = ctx,
                sources = direct_srcs,
                data = [],
                deps = ctx.attr.deps,
            ),
        ),
        OutputGroupInfo(types = direct_decls),
        js_info(
            declarations = direct_decls,
            sources = direct_srcs,
            transitive_declarations = js_lib_helpers.gather_transitive_declarations(
                declarations = dts_outs,
                targets = ctx.attr.deps,
            ),
            transitive_sources = js_lib_helpers.gather_transitive_sources(
                sources = js_outs,
                targets = ctx.attr.deps,
            ),
        ),
    ]

ts_proto_library = rule(
    implementation = _ts_proto_library_impl,
    attrs = {
        "proto": attr.label(
            doc = "proto_library to generate JS/DTS for",
            providers = [ProtoInfo],
            mandatory = True,
        ),
        "deps": attr.label_list(
            providers = [JsInfo],
            doc = "Other ts_proto_library rules.",
        ),
        "_protoc": attr.label(
            default = "@com_google_protobuf//:protoc",
            executable = True,
            cfg = "exec"
        ),
        "_protoc_gen_js": attr.label(
            default = "@//bazel/nodejs:protoc_gen_js",
            executable = True,
            cfg = "exec"
        ),
        "_grpc_tools_node_protoc": attr.label(
            default = "@//bazel/nodejs:grpc_tools_node_protoc",
            executable = True,
            cfg = "exec"
        ),
        "_grpc_tools_node_protoc_ts": attr.label(
            default = "@//bazel/nodejs:grpc_tools_node_protoc_ts",
            executable = True,
            cfg = "exec"
        ),
    },
    doc = "TODO",
)
