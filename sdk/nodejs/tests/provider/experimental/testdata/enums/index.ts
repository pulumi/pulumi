// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

/**
 * Represents the priority level of a component.
 */
export enum PriorityLevel {
    /** Low priority tasks */
    Low = "low",
    /** Normal priority tasks - the default */
    Medium = "medium",
    /** High priority tasks requiring immediate attention */
    High = "high",
    /** Critical tasks that must be processed first */
    Critical = "critical!",
}

/**
 * Configuration for a notification
 */
export interface NotificationConfig {
    /** Type of notification */
    type: pulumi.Input<NotificationType>;
    /** Recipients of the notification */
    recipients: string[];
    /** Notification only sent for these status changes */
    onlyForStatuses?: ResourceStatus[];
}

export interface MyComponentArgs {
    /** The status of the component */
    status?: pulumi.Input<ResourceStatus>;
    /** The priority level of the component */
    priority: PriorityLevel;
    /** Notification configuration */
    notifications?: NotificationConfig[];
}

/**
 * Status history entry
 */
export interface StatusHistoryEntry {
    /** The status value */
    status: pulumi.Input<ResourceStatus>;
    /** When the status was set */
    timestamp: pulumi.Input<string>;
    /** Priority at the time */
    priority: pulumi.Input<PriorityLevel | undefined>;
}

/**
 * Represents different types of notification channels.
 */
export enum NotificationType {
    /** Email notifications */
    Email = "email",
    /** SMS notifications */
    SMS = "sms",
    /** Webhook notifications */
    Webhook = "webhook",
    /** Push notifications */
    Push = "push",
}

export class MyComponent extends pulumi.ComponentResource {
    /** The current status of the resource */
    status: pulumi.Output<ResourceStatus>;
    /** The priority of the component */
    priority: pulumi.Output<PriorityLevel | undefined>;
    /** History of status changes */
    statusHistory: pulumi.Output<StatusHistoryEntry[]>;
    /** Notification configurations */
    notifications: pulumi.Output<NotificationConfig[] | undefined>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);

        this.status = pulumi.output(args.status || ResourceStatus.Provisioning);
        this.priority = pulumi.output(args.priority);
        this.notifications = pulumi.output(args.notifications);

        // Initial history entry
        this.statusHistory = pulumi.output([
            {
                status: this.status.get(),
                timestamp: new Date().toISOString(),
                priority: this.priority.get() || PriorityLevel.Medium,
            },
        ]);

        this.registerOutputs({
            status: this.status,
            priority: this.priority,
            statusHistory: this.statusHistory,
            notifications: this.notifications,
        });
    }
}

/**
 * Represents the status of a resource.
 * This enum demonstrates string-based enum definition.
 */
export enum ResourceStatus {
    /** Resource is being provisioned */
    Provisioning = "Provisioning",
    /** Resource is active and ready to use */
    Active = "Active",
    /** Resource is being deleted */
    Deleting = "Deleting",
    /** Resource is in a failed state */
    Failed = "Failed",
}
