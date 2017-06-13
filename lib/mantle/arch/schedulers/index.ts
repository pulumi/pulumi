// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The available container scheduler/runtimes.
export const swarm = "swarm";           // Docker Swarm.
export const kubernetes = "kubernetes"; // Kubernetes.
export const mesos = "mesos";           // Apache Mesos.
export const ecs = "ecs";               // Amazon Elastic Container Service (only valid for AWS clouds).
export const gke = "gke";               // Google Container Engine (only valid for GCP clouds).
export const acs = "acs";               // Microsoft Azure Container Service (only valid for Azure).

