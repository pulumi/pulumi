import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const names = config.getObject<Array<string>>("names") || [
    null,
    "hello",
    null,
];
export const namesLength = names.length;
