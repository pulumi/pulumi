import * as pulumi from "@pulumi/pulumi";

const myStash = new pulumi.Stash("myStash", {value: "ignored"});
export const stashOutput = myStash.value;
const passthroughStash = new pulumi.Stash("passthroughStash", {
    value: "new",
    passthrough: true,
});
export const passthroughOutput = passthroughStash.value;
