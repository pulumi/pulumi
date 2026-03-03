import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const aSecret = config.requireSecret("aSecret");
const notSecret = config.require("notSecret");
export const roundtripSecret = aSecret;
export const roundtripNotSecret = notSecret;
export const open = pulumi.unsecret(aSecret);
export const close = pulumi.secret(notSecret);
