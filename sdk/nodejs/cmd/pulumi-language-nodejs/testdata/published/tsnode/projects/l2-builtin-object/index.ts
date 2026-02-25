import * as pulumi from "@pulumi/pulumi";
import * as output from "@pulumi/output";

const res = new output.ComplexResource("res", {value: 1});
export const entriesOutput = res.outputMap.apply(outputMap => Object.entries(outputMap).map(([k, v]) => ({key: k, value: v})));
export const lookupOutput = res.outputMap.apply(outputMap => outputMap["x"] || "default");
export const lookupOutputDefault = res.outputMap.apply(outputMap => outputMap["y"] || "default");
