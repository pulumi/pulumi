// Copyright 2016 Pulumi, Inc. All rights reserved.

// The available container scheduler/runtimes.

// Docker Swarm.
export const Swarm = "swarm";
// Kubernetes.
export const Kubernetes = "kubernetes";
// Apache Mesos.
export const Mesos = "mesos";
// Amazon Elastic Container Service (only valid for AWS clouds).
export const ECS = "ecs";
// Google Container Engine (only valid for GCP clouds).
export const GKE = "gke";
// Microsoft Azure Container Service (only valid for Azure).
export const ACS = "acs";

