// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export class Component extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, undefined, opts, true);
    }

    getMessage(args: Component.GetMessageArgs): pulumi.Output<Component.GetMessageResult> {
        return pulumi.runtime.call("testcomponent:index:Component/getMessage", {
            "__self__": this,
            "echo": args.echo,
        }, this);
    }
}

export namespace Component {
    export interface GetMessageArgs {
        echo: pulumi.Input<string>;
    }

    export interface GetMessageResult {
        message: string;
    }
}
