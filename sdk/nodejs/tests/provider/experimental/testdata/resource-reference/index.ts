// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as tls from "@pulumi/tls";

export interface MyComponentArgs {
    /**
     * A reference to a resource in the TLS package.
     */
    inputResource: pulumi.Input<tls.PrivateKey>;
    inputPlainResource?: tls.PrivateKey;
    inputResourceOrUndefined: pulumi.Input<tls.PrivateKey | undefined>;
}

export class MyComponent extends pulumi.ComponentResource {
    public readonly outputResource: pulumi.Output<tls.PrivateKey>;
    public readonly outputPlainResource: tls.PrivateKey;
    public readonly outputResourceOrUndefined: pulumi.Output<tls.PrivateKey | undefined>;
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
