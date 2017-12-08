// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// State is a structure containing state associated with a resource.  This resource may have been serialized and
// deserialized, or snapshotted from a live graph of resource objects.  The value's state is not, however, associated
// with any runtime objects in memory that may be actively involved in ongoing computations.
type State struct {
	Type    tokens.Type // the resource's type.
	URN     URN         // the resource's object urn, a human-friendly, unique name for the resource.
	Custom  bool        // true if the resource is custom, managed by a plugin.
	Delete  bool        // true if this resource is pending deletion due to a replacement.
	ID      ID          // the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
	Inputs  PropertyMap // the resource's input properties (as specified by the program).
	Outputs PropertyMap // the resource's complete output state (as returned by the resource provider).
	Parent  URN         // an optional parent URN that this resource belongs to.
}

// NewState creates a new resource value from existing resource state information.
func NewState(t tokens.Type, urn URN, custom bool, del bool, id ID,
	inputs PropertyMap, outputs PropertyMap, parent URN) *State {
	contract.Assertf(t != "", "type was empty")
	contract.Assertf(custom || id == "", "is custom or had empty ID")
	contract.Assertf(inputs != nil, "inputs was non-nil")
	return &State{
		Type:    t,
		URN:     urn,
		Custom:  custom,
		Delete:  del,
		ID:      id,
		Inputs:  inputs,
		Outputs: outputs,
		Parent:  parent,
	}
}

// All returns all resource state, including the inputs, defaults, and outputs, overlaid in that order.
func (s *State) All() PropertyMap {
	return s.Inputs.Merge(s.Outputs)
}
