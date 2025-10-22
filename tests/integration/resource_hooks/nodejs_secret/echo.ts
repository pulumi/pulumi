// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface EchoArgs {
    echo: pulumi.Input<string>;
}

export class Echo extends pulumi.CustomResource {
    declare public readonly echo: pulumi.Output<string>;
    constructor(name: string, args: EchoArgs, opts?: pulumi.CustomResourceOptions) {
        const props = { echo: args.echo }
        super("testprovider:index:Echo", name, props, opts);
    }
}
