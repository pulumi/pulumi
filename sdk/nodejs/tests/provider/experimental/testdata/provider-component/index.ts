// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "../../../../..";

export interface MyComponentArgs {
    message: pulumi.Input<string>;
}

export class TestComponent extends pulumi.ComponentResource {
    public readonly messageBack: pulumi.Output<string>;
    public readonly notAnOutput: string;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider-component:index:TestComponent", name, args, opts);

        this.messageBack = pulumi.Output.create(`Hello, ${args.message}!`);
        this.notAnOutput = `Hello, ${args.message}!`;

        this.registerOutputs({
            messageBack: this.messageBack,
            notAnOutput: this.notAnOutput,
        });
    }
}
