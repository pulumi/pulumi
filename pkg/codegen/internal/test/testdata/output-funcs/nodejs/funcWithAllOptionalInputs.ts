import * as utilities from "./utilities";
import * as pulumi from "@pulumi/pulumi";
/**
 * Check codegen of functions with all optional inputs.
 */
export function funcWithAllOptionalInputs(args?: FuncWithAllOptionalInputsArgs, opts?: pulumi.InvokeOptions): Promise<FuncWithAllOptionalInputsResult> {
    args = args || {};
    if (!opts) {
        opts = {}
    }

    if (!opts.version) {
        opts.version = utilities.getVersion();
    }
    return pulumi.runtime.invoke("madeup-package:codegentest:funcWithAllOptionalInputs", {
        "a": args.a,
        "b": args.b,
    }, opts);
}

export interface FuncWithAllOptionalInputsArgs {
    /**
     * Property A
     */
    a?: string;
    /**
     * Property B
     */
    b?: string;
}

export interface FuncWithAllOptionalInputsResult {
    readonly r: string;
}

export function funcWithAllOptionalInputsOutput(args?: FuncWithAllOptionalInputsOutputArgs, opts?: pulumi.InvokeOptions): pulumi.Output<FuncWithAllOptionalInputsResult> {
    return pulumi.output(args).apply(a => funcWithAllOptionalInputs(a, opts))
}

export interface FuncWithAllOptionalInputsOutputArgs {
    /**
     * Property A
     */
    a?: string;
    /**
     * Property B
     */
    b?: string;
}
