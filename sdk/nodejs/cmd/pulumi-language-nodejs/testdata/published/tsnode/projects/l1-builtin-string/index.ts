import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const aString = config.require("aString");
export const lengthOutput = [...new Intl.Segmenter().segment(aString)].length;
export const splitOutput = aString.split("-");
export const joinOutput = aString.split("-").join("|");
export const interpolateOutput = `prefix-${aString}`;
