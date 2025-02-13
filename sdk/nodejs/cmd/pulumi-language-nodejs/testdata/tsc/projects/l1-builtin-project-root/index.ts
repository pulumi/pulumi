import * as pulumi from "@pulumi/pulumi";

export const projectRootOutput = pulumi.runtime.getProjectRoot();
