import * as utilities from "./utilities";
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
    return pulumi.runtime.invoke("madeup-package:codegentest:funcWithConstInput", {
        "plainInput": args.plainInput,
    }, opts);
}

export interface FuncWithConstInputArgs {
    plainInput?: "fixed";
}
