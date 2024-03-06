import * as pulumi from "@pulumi/pulumi";
import * as fs from "fs";

export const strVar = "foo";
export const arrVar = [
    "fizz",
    "buzz",
];
export const readme = fs.readFileSync("./Pulumi.README.md", "utf8");
