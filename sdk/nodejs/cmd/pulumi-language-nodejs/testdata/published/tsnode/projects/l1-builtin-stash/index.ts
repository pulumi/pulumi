import * as pulumi from "@pulumi/pulumi";

const myStash = new pulumi.Stash("myStash", {
    key: [
        "value",
        "s",
    ],
    "": false,
});
export const stashOutput = myStash.value;
