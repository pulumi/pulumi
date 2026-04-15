import * as pulumi from "@pulumi/pulumi";

export const stackOutput = pulumi.getStack();
export const projectOutput = pulumi.getProject();
export const organizationOutput = pulumi.getOrganization();
