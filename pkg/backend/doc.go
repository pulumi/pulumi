// Copyright 2016-2020, Pulumi Corporation.
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

// Package backend defines several core interfaces and logic common across pluggable backends.
//
// The following interfaces are defined:
// - The Backend interface, which is the contract between the Pulumi engine and pluggable backend implementations of the
// Pulumi Cloud Service.
// - The CancellationScope interface, used to provide a scoped source of cancellation and termination requests.
// - The CancellationScopeSource interface, used to provide a source for cancellation scopes.
// - The PolicyPack interface, used to manage policy against a pluggable backend.
// - The PolicyPackReference interface, used by backends to get a qualified name for a policy pack.
// - The SnapshotPersister interface, used to save and invalidate snapshots for a pluggable backend.
// - The SpecificDeploymentExporter interface, used to indicate if a backend supports exporting a specific deployment
// version.
// - The Stack interface, used to manage stacks of resources against a pluggable backend.
// - The StackReference interface, used by backends to get a qualified name for a stack.
// - The StackSummary interface, used to get a read-only summary of a stack.
//
// This package also contains the SnapshotManager, which is an implementation of the engine.SnapshotManager interface.
package backend
