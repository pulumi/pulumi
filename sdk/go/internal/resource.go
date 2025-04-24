// Copyright 2016-2023, Pulumi Corporation.
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

package internal

// Resource is a cloud resource managed by Pulumi.
//
// Inside this package, Resource is just a marker interface.
// See pulumi.Resource for the real definition.
type Resource interface {
	isResource()
}

// ResourceState is the internal object
// that should be embedded in the ResourceState type
// for this package to consider it a resource.
//
// See pulumi.ResourceState for more information.
type ResourceState struct{}

var _ Resource = ResourceState{}

// isResource is a marker method for the ResourceState type.
func (ResourceState) isResource() {}
