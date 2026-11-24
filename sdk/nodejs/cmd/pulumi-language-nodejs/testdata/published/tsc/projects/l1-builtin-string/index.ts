import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const aString = config.require("aString");
export const lengthResult = [...new Intl.Segmenter().segment(aString)].length;
export const splitResult = aString.split("-");
export const joinResult = aString.split("-").join("|");
export const interpolateResult = `prefix-${aString}`;
