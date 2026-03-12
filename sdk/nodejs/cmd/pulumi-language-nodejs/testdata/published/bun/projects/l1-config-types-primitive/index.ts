import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const aNumber = config.requireNumber("aNumber");
export const theNumber = aNumber + 1.25;
const optionalNumber = config.getNumber("optionalNumber") || 41;
export const defaultNumber = optionalNumber + 1;
const aString = config.require("aString");
export const theString = `${aString} World`;
const optionalString = config.get("optionalString") || "defaultStringValue";
export const defaultString = optionalString;
const aBool = config.requireBoolean("aBool");
export const theBool = !aBool && true;
const optionalBool = config.getBoolean("optionalBool") || false;
export const defaultBool = optionalBool;
