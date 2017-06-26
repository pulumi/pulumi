// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// The available container scheduler/runtimes.
export const swarm = "swarm";           // Docker Swarm.
export const kubernetes = "kubernetes"; // Kubernetes.
export const mesos = "mesos";           // Apache Mesos.
export const ecs = "ecs";               // Amazon Elastic Container Service (only valid for AWS clouds).
export const gke = "gke";               // Google Container Engine (only valid for GCP clouds).
export const acs = "acs";               // Microsoft Azure Container Service (only valid for Azure).

