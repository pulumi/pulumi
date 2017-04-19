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

// Runtime represents a supported application runtime to target.
// TODO: this should eventually probably involve a version number.
// TODO: eventually we want many more platforms: Ruby, Java, C#, and so on.
export type Runtime = "nodejs" | "python";

