// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface ComponentArgs {
    first: pulumi.Input<string>;
    second: pulumi.Input<string>;
}

export class Component extends pulumi.ComponentResource {
    constructor(name: string, args: ComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, args, opts, true);
    }

    getMessage(args: Component.GetMessageArgs): pulumi.Output<Component.GetMessageResult> {
        return pulumi.runtime.call("testcomponent:index:Component/getMessage", {
            "__self__": this,
            "name": args.name,
        }, this);
    }
}

export namespace Component {
    export interface GetMessageArgs {
        name: pulumi.Input<string>;
    }

    export interface GetMessageResult {
        message: string;
    }
}
