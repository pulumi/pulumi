import * as pulumi from "@pulumi/pulumi";

const myStash = new pulumi.Stash("myStash", {input: "ignored"});
export const stashInput = myStash.input;
export const stashOutput = myStash.output;
