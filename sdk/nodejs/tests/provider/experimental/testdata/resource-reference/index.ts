// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as tls from "@pulumi/tls";

export interface MyComponentArgs {
    /**
     * A reference to a resource in the TLS package.
     */
    inputResource: pulumi.Input<tls.PrivateKey>;
    inputPlainResource?: tls.PrivateKey;
}

export class MyComponent extends pulumi.ComponentResource {
    public readonly outputResource: pulumi.Output<tls.PrivateKey>;
    public readonly outputPlainResource: tls.PrivateKey;
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
