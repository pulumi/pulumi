// Copyright 2017 Pulumi, Inc. All rights reserved.

// The available container scheduler/runtimes.

export const Swarm = "swarm";           // Docker Swarm.
export const Kubernetes = "kubernetes"; // Kubernetes.
export const Mesos = "mesos";           // Apache Mesos.
export const ECS = "ecs";               // Amazon Elastic Container Service (only valid for AWS clouds).
export const GKE = "gke";               // Google Container Engine (only valid for GCP clouds).
export const ACS = "acs";               // Microsoft Azure Container Service (only valid for Azure).

