import * as pulumi from "@pulumi/pulumi";
import * as output from "@pulumi/output";
import * as simple from "@pulumi/simple";

// This test checks that when a provider doesn't return properties for fields it considers unknown the runtime
// can still access that field as an output.
const prov = new output.Provider("prov", {elideUnknowns: true});
const unknown = new output.Resource("unknown", {value: 1}, {
    provider: prov,
});
// Try and use the unknown output as an input to another resource to check that it doesn't cause any issues.
const res = new simple.Resource("res", {value: unknown.output.apply(output => output == "hello")});
export const out = unknown.output;
