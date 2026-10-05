import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";
import * as simple_invoke from "@pulumi/simple-invoke";

// Baseline for invoke dependency propagation: an invoke that reads properties from two different
// resources produces a return value whose consumer must depend on the union of both.
const a = new simple_invoke.StringResource("a", {text: "hello"});
const b = new simple.Resource("b", {value: true});
const data = simple_invoke.secretInvokeOutput({
    value: a.text,
    secretResponse: b.value,
});
const d = new simple_invoke.StringResource("d", {text: data.response});
