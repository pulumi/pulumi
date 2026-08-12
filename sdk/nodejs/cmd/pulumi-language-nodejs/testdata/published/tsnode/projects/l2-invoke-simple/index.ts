import * as pulumi from "@pulumi/pulumi";
import * as simple_invoke from "@pulumi/simple-invoke";

export const hello = simple_invoke.myInvokeOutput({
    value: "hello",
}).result;
export const goodbye = simple_invoke.myInvokeOutput({
    value: "goodbye",
}).result;
