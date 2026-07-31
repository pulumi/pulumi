import * as pulumi from "@pulumi/pulumi";
import * as call from "@pulumi/call";

const explicitProv = new call.Provider("explicitProv", {value: "explicitProvValue"});
const explicitRes = new call.Custom("explicitRes", {value: "explicitValue"}, {
    provider: explicitProv,
});
export const explicitProviderValue = explicitRes.providerValue().result;
export const explicitProvFromIdentity = explicitProv.identity().result;
export const explicitProvFromPrefixed = explicitProv.prefixed(({
    prefix: "call-prefix-",
})).result;
