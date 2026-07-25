// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as base from "@pulumi/basecomponent";

interface MyServiceArgs extends base.ServiceArgs {
    replicas: number;
}

export class MyService extends base.Service {
    endpoint: pulumi.Output<string>;

    constructor(name: string, args: MyServiceArgs, opts?: pulumi.ComponentResourceOptions) {
        super(name, args, opts);
    }
}
