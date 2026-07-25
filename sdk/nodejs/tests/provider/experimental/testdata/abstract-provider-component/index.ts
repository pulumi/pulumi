// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "../../../../..";

export interface AbstractComponentArgs {
    message: pulumi.Input<string>;
}

export abstract class AbstractComponent extends pulumi.ComponentResource {
    public readonly messageBack: pulumi.Output<string>;

    constructor(name: string, args: AbstractComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("abstract-provider-component:index:AbstractComponent", name, args, opts);

        this.messageBack = pulumi.Output.create(`Hello, ${args.message}!`);
        this.registerOutputs({ messageBack: this.messageBack });
    }
}
