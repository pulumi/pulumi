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

