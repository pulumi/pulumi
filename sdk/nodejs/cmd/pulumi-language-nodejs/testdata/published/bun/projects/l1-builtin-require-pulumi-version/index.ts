import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const version = config.require("version");
pulumi.requirePulumiVersion(version);
