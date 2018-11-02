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

package pulumi

type (
	// ID is a unique identifier assigned by a resource provider to a resource.
	ID string
	// URN is an automatically generated logical URN, used to stably identify resources.
	URN string
)

// Resource represents a cloud resource managed by Pulumi.
type Resource interface {
	// URN is this resource's stable logical URN used to distinctly address it before, during, and after deployments.
	URN() *URNOutput
}

// CustomResource is a cloud resource whose create, read, update, and delete (CRUD) operations are managed by performing
// external operations on some physical entity.  The engine understands how to diff and perform partial updates of them,
// and these CRUD operations are implemented in a dynamically loaded plugin for the defining package.
type CustomResource interface {
	Resource
	// ID is the provider-assigned unique identifier for this managed resource.  It is set during deployments,
	// but might be missing ("") during planning phases.
	ID() *IDOutput
}

// ComponentResource is a resource that aggregates one or more other child resources into a higher level abstraction.
// The component resource itself is a resource, but does not require custom CRUD operations for provisioning.
type ComponentResource interface {
	Resource
}

// ProviderResource is a resource that represents a configured instance of a particular package's provider plugin.
// These resources are supply the implementations of their package's CRUD operations. A specific provider instance can
// be used for a given resource by passing it in ResourceOpt.Provider.
type ProviderResource interface {
	CustomResource
}

// ResourceOpt contains optional settings that control a resource's behavior.
type ResourceOpt struct {
	// Parent is an optional parent resource to which this resource belongs.
	Parent Resource
	// DependsOn is an optional array of explicit dependencies on other resources.
	DependsOn []Resource
	// Protect, when set to true, ensures that this resource cannot be deleted (without first setting it to false).
	Protect bool
	// Provider is an optional provider resource to use for this resource's CRUD operations.
	Provider ProviderResource
}

// InvokeOpt contains optional settings that control an invoke's behavior.
type InvokeOpt struct {
	// Provider is an optional provider resource to use for this invoke.
	Provider ProviderResource
}
