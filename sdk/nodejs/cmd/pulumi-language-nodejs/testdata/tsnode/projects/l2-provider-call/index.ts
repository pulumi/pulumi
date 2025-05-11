import * as pulumi from "@pulumi/pulumi";
import * as call from "@pulumi/call";

const defaultRes = new call.Custom("defaultRes", {value: "defaultValue"});
export const defaultProviderValue = defaultRes.providerValue().apply(call => call.result);
