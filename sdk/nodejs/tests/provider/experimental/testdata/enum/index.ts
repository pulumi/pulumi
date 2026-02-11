// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

/** This demonstrates TypeScript string enums */
enum ResourceStatus {
    /** The provisioning status */
    Provisioning = "provisioning",
    /** The active status */
    Active = "active",
    /** The deleting status */
    Deleting = "deleting",
    /** The failed status */
    Failed = "failed",
}

/** This demonstrates TypeScript numeric enums */
enum Priority {
    Low = 0,
    Medium = 1,
    High = 2,
    Critical = 3,
}

export interface MyComponentArgs {
    /** The status of the component */
    status?: pulumi.Input<ResourceStatus>;
    /** The priority level */
    priority?: pulumi.Input<Priority>;
}

export class MyComponent extends pulumi.ComponentResource {
    /** The current status of the resource */
    status: pulumi.Output<ResourceStatus>;
    /** The priority of the resource */
    priority: pulumi.Output<Priority>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.status = pulumi.output(args.status || ResourceStatus.Provisioning);
        this.priority = pulumi.output(args.priority !== undefined ? args.priority : Priority.Medium);
        this.registerOutputs({
            status: this.status,
            priority: this.priority,
        });
    }
}
