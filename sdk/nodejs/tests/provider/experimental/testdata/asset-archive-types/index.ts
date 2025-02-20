// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    anAsset: pulumi.asset.Asset;
    anArchive: pulumi.asset.Archive;
    inputAsset: pulumi.Input<pulumi.asset.Asset>;
    inputArchive: pulumi.Input<pulumi.asset.Archive>;
}

export class MyComponent extends pulumi.ComponentResource {
    outAsset: pulumi.Output<pulumi.asset.Asset>;
    outArchive: pulumi.Output<pulumi.asset.Archive>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
