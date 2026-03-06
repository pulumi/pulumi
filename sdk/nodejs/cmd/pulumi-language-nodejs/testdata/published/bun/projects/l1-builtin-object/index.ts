import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const aMap = config.requireObject<Record<string, string>>("aMap");
export const entriesOutput = Object.entries(aMap).map(([k, v]) => ({key: k, value: v}));
export const lookupOutput = aMap["keyPresent"] || "default";
export const lookupOutputDefault = aMap["keyMissing"] || "default";
