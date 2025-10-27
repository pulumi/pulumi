import * as pulumi from "@pulumi/pulumi";

const myStash = new pulumi.Stash("myStash", {value: {
    key: [
        "value",
        "s",
    ],
    "": false,
}});
export const stashOutput = myStash.value;
const passthroughStash = new pulumi.Stash("passthroughStash", {
    value: "old",
    passthrough: true,
});
export const passthroughOutput = passthroughStash.value;
