// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class FooResource extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        const aliasOpts = { aliases: [{ type: "my:module:FooResource" }] };
        opts = pulumi.mergeOptions(opts, aliasOpts);
        super("my:module:FooResourceNew", name, {}, opts);
    }
}

class ComponentResource extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:ComponentResource", name, {}, opts);
        new FooResource("child", { parent: this });
    }
}

new ComponentResource("comp");
