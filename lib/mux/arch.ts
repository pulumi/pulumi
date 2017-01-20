// Copyright 2016 Marapongo, Inc. All rights reserved.

// An architecture is a combination of cloud plus optionally a scheduler that we're targeting.
export interface Arch {
    cloud: Cloud;
    scheduler: Scheduler;
}

// The cloud operating system to target.
// TODO: As soon as this PR is merged, https://github.com/Microsoft/TypeScript/pull/10676, I believe we can replace
//     these with references to the above Clouds literals (e.g., `typeof clouds.AWS`, etc).  For now, duplicate.
export type Cloud = "aws" | "gcp" | "azure" | "vmware";

// The container scheduler and runtime to target.
// TODO: As soon as this PR is merged, https://github.com/Microsoft/TypeScript/pull/10676, I believe we can replace
//     these with references to the above Clouds literals (e.g., `typeof schedulers.Swarm`, etc).  For now, duplicate.
export type Scheduler =
    undefined |                        // no scheduler, just use VMs.
    "swarm" | "kubernetes" | "mesos" | // cloud-neutral schedulers.
    "ecs" | "gke" | "acs";             // cloud-specific schedulers.

