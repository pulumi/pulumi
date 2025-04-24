import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";
import * as simple_invoke from "@pulumi/simple-invoke";

const res = new simple.Resource("res", {value: true});
export const nonSecret = simple_invoke.secretInvokeOutput({
    value: "hello",
    secretResponse: false,
}).apply(invoke => invoke.response);
export const firstSecret = simple_invoke.secretInvokeOutput({
    value: "hello",
    secretResponse: res.value,
}).apply(invoke => invoke.response);
export const secondSecret = simple_invoke.secretInvokeOutput({
    value: pulumi.secret("goodbye"),
    secretResponse: false,
}).apply(invoke => invoke.response);
