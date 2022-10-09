import * as pulumi from "@pulumi/pulumi";
import * as fs from "fs";

const key = fs.readFileSync("key.pub");
export const result = key;
