// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

/** This demonstrates const enums */
const ResourceStatus = {
    /** The provisioning status */
    Provisioning: "provisioning",
    /** The active status */
    Active: "active",
    /** The deleting status */
    Deleting: "deleting",
    /** The failed status */
    Failed: "failed",
} as const;
type ResourceStatus = (typeof ResourceStatus)[keyof typeof ResourceStatus];

export interface MyComponentArgs {
    /** The status of the component */
    status?: pulumi.Input<ResourceStatus>;
}

export class MyComponent extends pulumi.ComponentResource {
    /** The current status of the resource */
    status: pulumi.Output<ResourceStatus>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.status = pulumi.output(args.status || ResourceStatus.Provisioning);
        this.registerOutputs({
            status: this.status,
        });
    }
}
