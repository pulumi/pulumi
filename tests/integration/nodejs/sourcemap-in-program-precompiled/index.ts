import * as pulumi from "@pulumi/pulumi";

export function willThrow() {
    if (true) {
        pulumi.log.error("Oh no!");
        throw new Error("this is a test error");
    }
}

willThrow();
