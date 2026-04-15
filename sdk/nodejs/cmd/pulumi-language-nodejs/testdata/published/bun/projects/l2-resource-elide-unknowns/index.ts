import * as pulumi from "@pulumi/pulumi";
import * as output from "@pulumi/output";
import * as simple from "@pulumi/simple";

// This test checks that when a provider doesn't return properties for fields it considers unknown the runtime
// can still access that field as an output.
const prov = new output.Provider("prov", {elideUnknowns: true});
const unknown = new output.Resource("unknown", {value: 1}, {
    provider: prov,
});
const complex = new output.ComplexResource("complex", {value: 1}, {
    provider: prov,
});
// Try and use the unknown output as an input to another resource to check that it doesn't cause any issues.
const res = new simple.Resource("res", {value: unknown.output.apply(output => output == "hello")});
const resArray = new simple.Resource("resArray", {value: complex.outputArray.apply(outputArray => outputArray[0] == "hello")});
const resMap = new simple.Resource("resMap", {value: complex.outputMap.apply(outputMap => outputMap.x == "hello")});
const resObject = new simple.Resource("resObject", {value: complex.outputObject.apply(outputObject => outputObject.output == "hello")});
export const out = unknown.output;
export const outArray = complex.outputArray[0];
export const outMap = complex.outputMap.x;
export const outObject = complex.outputObject.output;
