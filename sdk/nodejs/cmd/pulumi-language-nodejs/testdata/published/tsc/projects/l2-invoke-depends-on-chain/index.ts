import * as pulumi from "@pulumi/pulumi";
import * as simple_invoke from "@pulumi/simple-invoke";

const first = new simple_invoke.StringResource("first", {text: "first"});
const second = new simple_invoke.StringResource("second", {text: "second"});
// getText fails unless a StringResource has already been created, so an SDK
// that drops the dependsOn option calls it during preview and fails the test.
const gated = simple_invoke.getTextOutput({
    text: "Goodbye",
}, {
    dependsOn: [first],
});
// myInvoke fails when called with an unknown argument, so an SDK that does not
// await the gated invoke before chaining calls it during preview and fails the
// test.
const chained = simple_invoke.myInvokeOutput({
    value: gated.result,
}, {
    dependsOn: [second],
});
export const result = chained.result;
