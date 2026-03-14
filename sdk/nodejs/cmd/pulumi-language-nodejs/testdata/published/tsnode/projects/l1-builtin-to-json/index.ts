import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const aString = config.require("aString");
const aNumber = config.requireNumber("aNumber");
const aList = config.requireObject<Array<string>>("aList");
const aSecret = config.requireSecret("aSecret");
export const stringOutput = JSON.stringify(aString);
export const numberOutput = JSON.stringify(aNumber);
export const boolOutput = JSON.stringify(true);
export const arrayOutput = JSON.stringify([
    "x",
    "y",
    "z",
]);
export const objectOutput = JSON.stringify({
    key: "value",
    count: 1,
});
// Nested object using config values
const nestedObject = {
    anObject: {
        name: aString,
        items: aList,
    },
    a_secret: aSecret,
};
export const nestedOutput = pulumi.jsonStringify(nestedObject);
