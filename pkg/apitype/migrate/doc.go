// Copyright 2016-2018, Pulumi Corporation.
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

// Package migrate is responsible for converting to and from the various API
// type versions that are in use in Pulumi. This package can migrate "up" for
// every versioned API that needs to be migrated between versions. Today, there
// are three versionable entities that can be migrated
// with this package:
//   * Checkpoint, the on-disk format for Fire-and-Forget stack state,
//   * Deployment, the wire format for service-managed stacks,
//   * Resource, the wire format for resources saved in deployments,
//
// The migrations in this package are designed to preserve semantics between
// versions. It is always safe to migrate an entity up from one version to another.
package migrate
