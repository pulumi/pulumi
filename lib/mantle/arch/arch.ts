// Copyright 2017 Pulumi, Inc. All rights reserved.

// Arch is a combination of cloud plus optionally a scheduler that we're targeting.
export interface Arch {
    cloud: Cloud;
    scheduler?: Scheduler;
}

// Cloud is the cloud provider to target.
export type Cloud = "aws" | "gcp" | "azure" | "vmware";

// Scheduler is the container scheduler and runtime to target.
export type Scheduler =
    "kubernetes" | "mesos" | "swarm" | // cloud-neutral schedulers.
    "ecs" | "gke" | "acs";             // cloud-specific schedulers.

