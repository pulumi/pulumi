import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const value = config.require("value");
const tags = config.getObject("tags") || {
    [`interpolated/${value}`]: "value",
};
