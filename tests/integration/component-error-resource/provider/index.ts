// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as command from "@pulumi/command";

interface Nested {
    aNumber: number;
}

interface Complex {
    aNumber: number;
    nestedComplexType: Nested;
}

interface MyComponentArgs {
}

export class MyComponent extends pulumi.ComponentResource {
    aString: pulumi.Output<string>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("nodejs-component-provider:index:MyComponent", name, args, opts);
        const c = new command.local.Command("curl", {
            "create": "echo 'exiting with error' && exit 1"
        });
        this.aString = c.stdout;
        this.registerOutputs({
            aString: this.aString,
        });
    }
}
