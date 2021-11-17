// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface BarArgs {
    tags?: pulumi.Input<{[key: string]: pulumi.Input<string>}>;
}

export interface FooArgs {
    something?: pulumi.Input<string>;
}

export interface ComponentArgs {
    bar?: pulumi.Input<BarArgs>;
    foo?: FooArgs;
}

export class Component extends pulumi.ComponentResource {
    constructor(name: string, args?: ComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, args, opts, true /*remote*/);
    }
}
