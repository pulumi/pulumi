// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

interface ComponentArgs {
    echo: pulumi.Input<any>;
}

export class Component extends pulumi.ComponentResource {
    public readonly echo!: pulumi.Output<any>;
    public readonly childId!: pulumi.Output<pulumi.ID>;

    constructor(name: string, args: ComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        const inputs: any = {};
        inputs["echo"] = args.echo;
        inputs["childId"] = undefined /*out*/;
        inputs["secret"] = undefined /*out*/;

        super("testcomponent:index:Component", name, inputs, opts, true);
    }
}

