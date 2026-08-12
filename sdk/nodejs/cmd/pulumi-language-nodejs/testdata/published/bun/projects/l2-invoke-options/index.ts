import * as pulumi from "@pulumi/pulumi";
import * as simple_invoke from "@pulumi/simple-invoke";

const explicitProvider = new simple_invoke.Provider("explicitProvider", {});
const data = simple_invoke.myInvokeOutput({
    value: "hello",
}, {
    provider: explicitProvider,
    parent: explicitProvider,
    version: "10.0.0",
    pluginDownloadURL: "https://example.com/github/example",
});
export const hello = data.result;
