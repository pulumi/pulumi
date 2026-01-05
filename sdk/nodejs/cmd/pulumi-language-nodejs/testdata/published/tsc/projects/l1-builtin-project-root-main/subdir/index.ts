import * as pulumi from "@pulumi/pulumi";

export const rootDirectoryOutput = pulumi.getRootDirectory();
export const workingDirectoryOutput = process.cwd();
