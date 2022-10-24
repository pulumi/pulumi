import * as pulumi from "@pulumi/pulumi";
import * as fs from "fs";

const basicStrVar = "foo";
export const strVar = basicStrVar;
export const computedStrVar = `${basicStrVar}/computed`;
export const strArrVar = [
    "fiz",
    "buss",
];
export const intVar = 42;
export const intArr = [
    1,
    2,
    3,
    4,
    5,
];
export const readme = fs.readFileSync("./Pulumi.README.md");
