import * as pulumi from "@pulumi/pulumi";
import * as output_only_invoke from "@pulumi/output-only-invoke";

export const hello = output_only_invoke.myInvokeOutput({
    value: "hello",
}).result;
export const goodbye = output_only_invoke.myInvokeOutput({
    value: "goodbye",
}).result;
