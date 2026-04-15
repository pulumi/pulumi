import * as pulumi from "@pulumi/pulumi";
import * as simple_invoke from "@pulumi/simple-invoke";

const res = new simple_invoke.StringResource("res", {text: "hello"});
export const outputInput = simple_invoke.myInvokeOutput({
    value: res.text,
}).apply(invoke => invoke.result);
export const unit = simple_invoke.unitOutput({}).apply(invoke => invoke.result);
