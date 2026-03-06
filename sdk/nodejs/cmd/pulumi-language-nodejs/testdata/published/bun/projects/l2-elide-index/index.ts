import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";
import * as simple_invoke from "@pulumi/simple-invoke";

const res = new simple.Resource("res", {value: true});
export const inv = simple_invoke.myInvokeOutput({
    value: "test",
}).apply(invoke => invoke.result);
