// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as process from "node:process";
import * as pulumi from "@pulumi/pulumi";
import { log, ResourceHookArgs } from "@pulumi/pulumi";

interface EchoArgs {
    echo: pulumi.Input<string>;
}

class Echo extends pulumi.CustomResource {
    declare public readonly echo: pulumi.Output<string>;
    constructor(name: string, args: EchoArgs, opts?: pulumi.CustomResourceOptions) {
        super("testprovider:index:Echo", name, { echo: args.echo }, opts);
    }
}

async function hook(_args: ResourceHookArgs) {
    log.info("hook called");
}

new Echo("echo", { echo: "hello" }, {
    hooks: {
        beforeCreate: [hook],
    },
});

export const isDeno = process.versions.deno !== undefined;
