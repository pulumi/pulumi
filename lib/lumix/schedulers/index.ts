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

