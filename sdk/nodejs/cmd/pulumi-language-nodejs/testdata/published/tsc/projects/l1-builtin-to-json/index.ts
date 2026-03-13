import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const aString = config.require("aString");
const aNumber = config.requireNumber("aNumber");
const aList = config.requireObject<Array<string>>("aList");
const aSecret = config.requireSecret("aSecret");
// Literal data shapes built as locals
const literalBool = true;
const literalArray = [
    "x",
    "y",
    "z",
];
const literalObject = {
    key: "value",
    count: 1,
};
// Nested object using config values
const nestedObject = {
    name: aString,
    items: aList,
    a_secret: aSecret,
};
export const stringOutput = JSON.stringify(aString);
export const numberOutput = JSON.stringify(aNumber);
export const boolOutput = JSON.stringify(literalBool);
export const arrayOutput = JSON.stringify(literalArray);
export const objectOutput = JSON.stringify(literalObject);
export const nestedOutput = pulumi.jsonStringify(nestedObject);
