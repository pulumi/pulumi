// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class FooResource extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:FooResource", name, {}, opts);
    }
}

class ComponentResource extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:ComponentResource", name, {}, opts);
        new FooResource("childRetypeChild", { parent: this });
    }
}

new ComponentResource("compRetypeChild");
