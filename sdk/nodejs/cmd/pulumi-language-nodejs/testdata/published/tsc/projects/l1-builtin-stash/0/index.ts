import * as pulumi from "@pulumi/pulumi";

const myStash = new pulumi.Stash("myStash", {input: {
    key: [
        "value",
        "s",
    ],
    "": false,
}});
export const stashInput = myStash.input;
export const stashOutput = myStash.output;
const passthroughStash = new pulumi.Stash("passthroughStash", {
    input: "old",
    passthrough: true,
});
export const passthroughInput = passthroughStash.input;
export const passthroughOutput = passthroughStash.output;
