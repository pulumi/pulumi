import * as pulumi from "@pulumi/pulumi";
import * as simple_invoke from "@pulumi/simple-invoke";

const myInvokeResult = simple_invoke.myInvokeOutput({
    value: "hello",
});
const config = new pulumi.Config();
const defaultFromInvoke = config.get("defaultFromInvoke") || myInvokeResult.result;
export const result = defaultFromInvoke;
