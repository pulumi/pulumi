import * as pulumi from "@pulumi/pulumi";
import * as module_format from "@pulumi/module-format";

// This tests that PCL allows both fully specified type tokens, and tokens that only specify the module and
// member name.
// First use the fully specified token to invoke and create a resource.
const res1 = new module_format.mod.Resource("res1", {text: module_format.mod.concatWorldOutput({
    value: "hello",
}).apply(invoke => invoke.result)});
export const out1 = res1.call(({
    input: "x",
})).apply(call => call.output);
// Next use just the module name as defined by the module format
const res2 = new module_format.mod.Resource("res2", {text: module_format.mod.concatWorldOutput({
    value: "goodbye",
}).apply(invoke => invoke.result)});
export const out2 = res2.call(({
    input: "xx",
})).apply(call => call.output);
