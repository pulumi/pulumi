import * as pulumi from "@pulumi/pulumi";
import * as simple_invoke from "@pulumi/simple-invoke";

const provider = new simple_invoke.Provider("provider", {});
const parent = new simple_invoke.StringResource("parent", {text: "parent"}, {
    provider: provider,
});
// This should inherit the explicit provider from parent.
const child = new simple_invoke.StringResource("child", {text: "child"}, {
    parent: parent,
});
// This should use the explicit provider of parent.
const parentData = simple_invoke.myInvokeOutput({
    value: "parent",
}, {
    parent: parent,
});
// This should use the explicit provider that child inherited from parent.
const childData = simple_invoke.myInvokeOutput({
    value: "child",
}, {
    parent: child,
});
export const parentResult = parentData.result;
export const childResult = childData.result;
