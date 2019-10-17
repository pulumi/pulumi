import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();

export const out = config.requireSecret("mysecret");
