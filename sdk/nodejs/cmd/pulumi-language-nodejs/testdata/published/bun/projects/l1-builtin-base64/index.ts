import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const input = config.require("input");
const bytes = Buffer.from(input, "base64").toString("utf8");
export const data = bytes;
export const roundtrip = Buffer.from(bytes).toString("base64");
