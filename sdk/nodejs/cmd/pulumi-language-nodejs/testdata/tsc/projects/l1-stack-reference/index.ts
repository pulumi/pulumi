import * as pulumi from "@pulumi/pulumi";

const ref = new pulumi.StackReference("ref", {name: "organization/other/dev"});
export const plain = ref.getOutput("plain");
export const secret = ref.getOutput("secret");
