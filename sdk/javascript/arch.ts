// Copyright 2016 Marapongo, Inc. All rights reserved.

// An architecture is a combination of cloud plus optionally a scheduler that we're targeting.
export interface Arch {
    cloud: Cloud;
    scheduler: Scheduler;
}

// The cloud operating system to target.
export type Cloud =
    "aws" |   // Amazon Web Services.
    "gcp" |   // Google Cloud Platform.
    "azure" | // Microsoft Azure.
    "vmware"  // VMWare vSphere, etc.
;

// The container scheduler and runtime to target.
export type Scheduler =
    undefined |    // no scheduler, just use native VMs.
    "swarm" |      // Docker Swarm.
    "kubernetes" | // Kubernetes.
    "mesos" |      // Apache Mesos.
    "ecs" |        // Amazon Elastic Container Service (only valid for AWS clouds).
    "gke" |        // Google Container Engine (only valid for GCP clouds).
    "acs"          // Microsoft Azure Container Service (only valid for Azure).
;

