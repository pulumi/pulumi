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

package resource

import (
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// State is a structure containing state associated with a resource.  This resource may have been serialized and
// deserialized, or snapshotted from a live graph of resource objects.  The value's state is not, however, associated
// with any runtime objects in memory that may be actively involved in ongoing computations.
// nolint: lll
type State struct {
	Type                 tokens.Type           // the resource's type.
	URN                  URN                   // the resource's object urn, a human-friendly, unique name for the resource.
	Custom               bool                  // true if the resource is custom, managed by a plugin.
	Delete               bool                  // true if this resource is pending deletion due to a replacement.
	ID                   ID                    // the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
	Inputs               PropertyMap           // the resource's input properties (as specified by the program).
	Outputs              PropertyMap           // the resource's complete output state (as returned by the resource provider).
	Parent               URN                   // an optional parent URN that this resource belongs to.
	Protect              bool                  // true to "protect" this resource (protected resources cannot be deleted).
	External             bool                  // true if this resource is "external" to Pulumi and we don't control the lifecycle
	Dependencies         []URN                 // the resource's dependencies
	InitErrors           []string              // the set of errors encountered in the process of initializing resource.
	Provider             string                // the provider to use for this resource.
	PropertyDependencies map[PropertyKey][]URN // the set of dependencies that affect each property.
	PendingReplacement   bool                  // true if this resource was deleted and is awaiting replacement.
}

// NewState creates a new resource value from existing resource state information.
func NewState(t tokens.Type, urn URN, custom bool, del bool, id ID,
	inputs PropertyMap, outputs PropertyMap, parent URN, protect bool,
	external bool, dependencies []URN, initErrors []string, provider string,
	propertyDependencies map[PropertyKey][]URN, pendingReplacement bool) *State {

	contract.Assertf(t != "", "type was empty")
	contract.Assertf(custom || id == "", "is custom or had empty ID")
	contract.Assertf(inputs != nil, "inputs was non-nil")
	return &State{
		Type:                 t,
		URN:                  urn,
		Custom:               custom,
		Delete:               del,
		ID:                   id,
		Inputs:               inputs,
		Outputs:              outputs,
		Parent:               parent,
		Protect:              protect,
		External:             external,
		Dependencies:         dependencies,
		InitErrors:           initErrors,
		Provider:             provider,
		PropertyDependencies: propertyDependencies,
		PendingReplacement:   pendingReplacement,
	}
}

// All returns all resource state, including the inputs and outputs, overlaid in that order.
func (s *State) All() PropertyMap {
	return s.Inputs.Merge(s.Outputs)
}
