import * as pulumi from "@pulumi/pulumi";

export const rootDirectoryOutput = pulumi.runtime.getRootDirectory();
