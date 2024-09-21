// Copyright 2021-2024, Pulumi Corporation.
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

package plugin

/*
Package plugin defines the plugin interfaces used by the Pulumi engine and provides basic plugin management for
out-of-process plugin implementations that communicate over gRPC.

Providers

The Provider type defines the interface that must be implemented by a Pulumi resource provider. Resource providers
use the resource.PropertyValue types as their basic format for exchanging data with the engine. These types represent
a superset of the value representable in JSON; they are documented in their containing package.

Over time, the lifecycle of a resource provider has become more complex, and the Provider interface has accumulated
a number of quirks intended to retain backwards compatibility with older versions of the Pulumi engine. These factors
combined make it difficult to understand what exactly the requirements are for a provider that targets a particular
version of the plugin interface. The lifecycle and interface requirements are described in
provider-implementers-guide.md.

Analyzers

TBD

Language Hosts

TBD
*/
