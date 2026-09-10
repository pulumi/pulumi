import * as pulumi from "@pulumi/pulumi";
import * as simple_invoke from "@pulumi/simple-invoke";

export const result = simple_invoke.invokeWithDefaultOutput({}).result;
export const explicitResult = simple_invoke.invokeWithDefaultOutput({
    value: "explicit",
}).result;
