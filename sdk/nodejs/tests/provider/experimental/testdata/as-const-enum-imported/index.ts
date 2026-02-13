// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import { DeploymentMode, RetryCount } from "./enums";

export interface MyComponentArgs {
    /** The deployment mode for the component */
    mode?: pulumi.Input<DeploymentMode>;
    /** The retry count for the component */
    retries?: pulumi.Input<RetryCount>;
}

export class MyComponent extends pulumi.ComponentResource {
    /** The current deployment mode */
    mode: pulumi.Output<DeploymentMode>;
    /** The current retry count */
    retries: pulumi.Output<RetryCount>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.mode = pulumi.output(args.mode || "dev");
        this.retries = pulumi.output(args.retries !== undefined ? args.retries : 5);
        this.registerOutputs({
            mode: this.mode,
            retries: this.retries,
        });
    }
}
