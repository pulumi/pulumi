load("@rules_proto//proto:defs.bzl", "ProtoInfo", "proto_common")

load("@aspect_bazel_lib//lib:copy_to_bin.bzl", "COPY_FILE_TO_BIN_TOOLCHAINS")
load("@aspect_rules_js//js:libs.bzl", "js_lib_helpers")
load("@aspect_rules_js//js:providers.bzl", "JsInfo", "js_info")

def _protoc_action(ctx, proto_info, outputs):
    inputs = depset(
        proto_info.direct_sources,
        transitive = [proto_info.transitive_descriptor_sets]
    )

    args = ctx.actions.args()

    args.add("--protoc", ctx.executable._protoc.path)
    args.add("--protoc-gen-grpc", ctx.executable._grpc_tools_node_protoc.path)
    args.add("--protoc-gen-js", ctx.executable._protoc_gen_js.path)
    args.add("--protoc-gen-ts", ctx.executable._grpc_tools_node_protoc_ts.path)

    args.add("--output-directory", ctx.bin_dir.path)
    args.add_all("--descriptor-sets", proto_info.transitive_descriptor_sets)
    args.add_all("--proto-files", proto_info.direct_sources)
    args.add_all("--output-files", outputs)

    ctx.actions.run(
        executable = ctx.executable._protoc_wrapper,
        progress_message = "Generating .js/.d.ts from %{label}",
        outputs = outputs,
        inputs = inputs,
        arguments = [args],
        mnemonic = "TsProtoLibrary",
        tools = [
            ctx.executable._protoc_wrapper,
            ctx.executable._protoc,
            ctx.executable._protoc_gen_js,
            ctx.executable._grpc_tools_node_protoc,
            ctx.executable._grpc_tools_node_protoc_ts,
        ],
        env = {
            "BAZEL_BINDIR": ctx.bin_dir.path,
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
        "_protoc_wrapper": attr.label(
            default = "@//bazel/nodejs:protoc_wrapper",
            executable = True,
            cfg = "exec"
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
