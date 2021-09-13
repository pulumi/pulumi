import * as utilities from "./utilities";
import * as pulumi from "@pulumi/pulumi";
/**
 * Check codegen of functions with a List parameter.
 */
export function funcWithListParam(args?: FuncWithListParamArgs, opts?: pulumi.InvokeOptions): Promise<FuncWithListParamResult> {
    args = args || {};
    if (!opts) {
        opts = {}
    }

    if (!opts.version) {
        opts.version = utilities.getVersion();
    }
    return pulumi.runtime.invoke("madeup-package:codegentest:funcWithListParam", {
        "a": args.a,
        "b": args.b,
    }, opts);
}

export interface FuncWithListParamArgs {
    a?: string[];
    b?: string;
}

export interface FuncWithListParamResult {
    readonly r: string;
}

export function funcWithListParamOutput(args?: FuncWithListParamOutputArgs, opts?: pulumi.InvokeOptions): pulumi.Output<FuncWithListParamResult> {
    return pulumi.output(args).apply(a => funcWithListParam(a, opts))
}

export interface FuncWithListParamOutputArgs {
    a?: pulumi.Input<pulumi.Input<string>[]>;
    b?: pulumi.Input<string>;
}
