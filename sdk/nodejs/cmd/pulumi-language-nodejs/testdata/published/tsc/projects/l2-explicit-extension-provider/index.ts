import * as pulumi from "@pulumi/pulumi";
import * as extbase from "@pulumi/extbase";
import * as myext from "@pulumi/myext";

const prov = new extbase.Provider("prov", {});
const greeting = new myext.Greeting("greeting", {}, {
    provider: prov,
});
const base = new extbase.Base("base", {}, {
    provider: prov,
});
export const parameterValue = greeting.parameterValue;
export const baseValue = base.baseValue;
