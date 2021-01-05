// Copyright 2016-2021, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package deploy contains the logic for planning and executing resource deployments. This includes the following:
// - Manage the deployment lifecycle by coordinating the execution and parallelism of the underlying operations.
// - A builtin provider for interacting with the engine.
//
// The following interfaces are defined:
// - The BackendClient interface, used to retrieve information about stacks from a backend.
// - The Events interface, used to hook engine events.
// - The PolicyEvents interface, used to hook policy events.
// - The ProviderSource interface, used to look up provider plugins.
// - The QuerySource interface, used to synchronously wait for a query result.
// - The ReadResourceEvent interface, which defines a step to read the state of an existing resource.
// - The RegisterResourceEvent interface, which defines a step to provision a resource.
// - The RegisterResourceOutputsEvent interface, which defines a step to complete provisioning of a resource.
// - The Source interface, used to generate a set of resources for the planner.
// - The SourceIterator interface, used to enumerate a list of resources for a Source.
// - The SourceResourceMonitor interface, used to direct resource operations from a Source to resource providers.
// - The StepExecutorEvents interface, used to hook resource lifecycle events.
package deploy
