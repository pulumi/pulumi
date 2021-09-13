import * as utilities from "./utilities";
import * as pulumi from "@pulumi/pulumi";
/**
 * Check codegen of functions with a Dict<str,str> parameter.
 */
export function funcWithDictParam(args?: FuncWithDictParamArgs, opts?: pulumi.InvokeOptions): Promise<FuncWithDictParamResult> {
    args = args || {};
    if (!opts) {
        opts = {}
    }

    if (!opts.version) {
        opts.version = utilities.getVersion();
    }
    return pulumi.runtime.invoke("madeup-package:codegentest:funcWithDictParam", {
        "a": args.a,
        "b": args.b,
    }, opts);
}

export interface FuncWithDictParamArgs {
    a?: {[key: string]: string};
    b?: string;
}

export interface FuncWithDictParamResult {
    readonly r: string;
}

export function funcWithDictParamOutput(args?: FuncWithDictParamOutputArgs, opts?: pulumi.InvokeOptions): pulumi.Output<FuncWithDictParamResult> {
    return pulumi.output(args).apply(a => funcWithDictParam(a, opts))
}

export interface FuncWithDictParamOutputArgs {
    a?: {[key: string]: string};
    b?: string;
}
