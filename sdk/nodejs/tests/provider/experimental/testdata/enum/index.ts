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

/** This demonstrates TypeScript numeric enums with computed values */
enum Level {
    /** Starting at 2 */
    A = 2,
    /** Auto-incremented to 3 */
    B,
    /** Auto-incremented to 4 */
    C,
}

export interface MyComponentArgs {
    /** The status of the component */
    status?: pulumi.Input<ResourceStatus>;
    /** The priority level */
    priority?: pulumi.Input<Priority>;
    /** The level */
    level?: pulumi.Input<Level>;
}

export class MyComponent extends pulumi.ComponentResource {
    /** The current status of the resource */
    status: pulumi.Output<ResourceStatus>;
    /** The priority of the resource */
    priority: pulumi.Output<Priority>;
    /** The level of the resource */
    level: pulumi.Output<Level>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
        this.status = pulumi.output(args.status || ResourceStatus.Provisioning);
        this.priority = pulumi.output(args.priority !== undefined ? args.priority : Priority.Medium);
        this.level = pulumi.output(args.level !== undefined ? args.level : Level.A);
        this.registerOutputs({
            status: this.status,
            priority: this.priority,
            level: this.level,
        });
    }
}
