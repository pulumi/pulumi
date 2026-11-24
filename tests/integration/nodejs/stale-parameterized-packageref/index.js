// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

const projectName = "stale-parameterized-packageref";

async function preview(stackName, program) {
    const stack = await pulumi.automation.LocalWorkspace.createOrSelectStack({
        stackName,
        projectName,
        program,
    });
    try {
        await stack.preview();
    } finally {
        await stack.workspace.removeStack(stackName).catch(() => { });
    }
}

await preview("stack-1", async () => {
    new random.Password("my-password", { length: 16 });
});
console.log("First preview succeeded");

await preview("stack-2", async () => {
    new random.Uuid("my-uuid");
});
console.log("Second preview succeeded");
