// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface EchoArgs {
    echo: pulumi.Input<any>;
}

export class Echo extends pulumi.CustomResource {
    public readonly echo!: pulumi.Output<any>;
    constructor(name: string, args: EchoArgs, opts?: pulumi.CustomResourceOptions) {
        super("testprovider:index:Echo", name, args, opts);
    }
}
