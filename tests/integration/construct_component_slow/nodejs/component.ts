// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export class Component extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, {}, opts, true);
    }
}
