import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const aNumber = config.requireNumber("aNumber");
export const theNumber = aNumber + 1.25;
const aString = config.require("aString");
export const theString = `${aString} World`;
const aBool = config.requireBoolean("aBool");
export const theBool = !aBool && true;
