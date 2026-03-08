import * as pulumi from "@pulumi/pulumi";
import * as goodbye from "@pulumi/goodbye";

const prov = new goodbye.Provider("prov", {text: "World"});
// The resource name is based on the parameter value
const res = new goodbye.Goodbye("res", {}, {
    provider: prov,
});
export const parameterValue = res.parameterValue;
