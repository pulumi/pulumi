// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export class Component extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, undefined, opts, true);
    }

    createRandom(args: Component.CreateRandomArgs): pulumi.Output<Component.CreateRandomResult> {
        return pulumi.runtime.call("testcomponent:index:Component/createRandom", {
            "__self__": this,
            "length": args.length,
        }, this);
    }
}

export namespace Component {
    export interface CreateRandomArgs {
        length: pulumi.Input<number>;
    }

    export interface CreateRandomResult {
        result: string;
    }
}
