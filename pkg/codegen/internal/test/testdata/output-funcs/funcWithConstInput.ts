import * as pulumi from "@pulumi/pulumi";
/**
 * Codegen demo with const inputs
 */
export function funcWithConstInput(args?: FuncWithConstInputArgs, opts?: pulumi.InvokeOptions): Promise<void> {
    args = args || {};
    if (!opts) {
        opts = {}
    }

    if (!opts.version) {
        opts.version = utilities.getVersion();
    }
    return pulumi.runtime.invoke("madeup-package:x:funcWithConstInput", {
        "plainInput": args.plainInput,
    }, opts);
}

export interface FuncWithConstInputArgs {
    plainInput?: "fixed";
}

export function funcWithConstInputOutput(args?: FuncWithConstInputOutputArgs, opts?: pulumi.InvokeOptions): pulumi.Output<void> {
    return pulumi.output(args).apply(a => funcWithConstInput(a, opts))
}

export interface FuncWithConstInputOutputArgs {
    plainInput?: pulumi.Input<"fixed">;
}
