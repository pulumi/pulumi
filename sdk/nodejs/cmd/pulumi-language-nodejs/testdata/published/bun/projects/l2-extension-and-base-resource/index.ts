import * as pulumi from "@pulumi/pulumi";
import * as extbase from "@pulumi/extbase";
import * as myext from "@pulumi/myext";

const greeting = new myext.Greeting("greeting", {});
const base = new extbase.Base("base", {});
export const parameterValue = greeting.parameterValue;
export const baseValue = base.baseValue;
