import * as pulumi from "@pulumi/pulumi";

const foo = pulumi.secret({
    superSecret: "shh",
});
export const anOutputTuple = [foo.superSecret];
