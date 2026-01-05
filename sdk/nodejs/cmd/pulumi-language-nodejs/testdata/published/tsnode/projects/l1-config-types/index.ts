import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const aNumber = config.requireNumber("aNumber");
export const theNumber = aNumber + 1.25;
const aString = config.require("aString");
export const theString = aString + " World";
const aMap = config.requireObject<Record<string, number>>("aMap");
export const theMap = {
    a: aMap.a + 1,
    b: aMap.b + 1,
};
const anObject = config.requireObject<{prop?: Array<boolean>}>("anObject");
export const theObject = anObject.prop?.[0];
const anyObject = config.requireObject<any>("anyObject");
export const theThing = anyObject.a + anyObject.b;
