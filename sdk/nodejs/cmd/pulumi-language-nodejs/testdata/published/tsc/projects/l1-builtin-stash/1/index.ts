import * as pulumi from "@pulumi/pulumi";

const myStash = new pulumi.Stash("myStash", {input: "ignored"});
export const stashInput = myStash.input;
export const stashOutput = myStash.output;
const passthroughStash = new pulumi.Stash("passthroughStash", {
    input: "new",
    passthrough: true,
});
export const passthroughInput = passthroughStash.input;
export const passthroughOutput = passthroughStash.output;
