// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface EchoArgs {
    echo: pulumi.Input<string>;
}

class Echo extends pulumi.CustomResource {
    declare public readonly echo: pulumi.Output<string>;
    constructor(name: string, args: EchoArgs, opts?: pulumi.CustomResourceOptions) {
        const props = { echo: args.echo };
        super("testprovider:index:Echo", name, props, opts);
    }
}

const res = new Echo("res", { echo: "hello" });

export const name = res.echo;
export const isDeno = typeof Deno !== "undefined";
