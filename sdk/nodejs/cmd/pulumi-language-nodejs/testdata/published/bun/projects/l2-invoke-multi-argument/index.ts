import * as pulumi from "@pulumi/pulumi";
import * as multi_argument_invoke from "@pulumi/multi-argument-invoke";

export const both = multi_argument_invoke.multiArgumentInvokeOutput("hello", "world").apply(invoke => invoke.result);
export const onlyRequired = multi_argument_invoke.multiArgumentInvokeOutput("hello").apply(invoke => invoke.result);
