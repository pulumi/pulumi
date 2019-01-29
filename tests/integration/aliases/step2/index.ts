// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Resource extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:Type", name, {}, opts);
    }
}

// This resource was previously named `foobar`, we'll alias to the old name.
const stackName = pulumi.getStack();
const projectName = pulumi.getProject();
const res = new Resource("baz", { aliases: [`urn:pulumi:${stackName}::${projectName}::my:module:Type::foobar`]});
