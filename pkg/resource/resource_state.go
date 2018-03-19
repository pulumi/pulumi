// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package resource

import (
	"time"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// MutationStatus is a status attached to a resource that indicates the liveness
// state of that resource. A resource is "live" if it is "creating" or "updating";
// all other states are considered to be not live for the purposes of the engine.
//
// As plans execute and the resource snapshot is mutated, the states of resources
// within the snapshot will change to reflect the changes being made.
type MutationStatus string

const (
	// ResourceStatusUnspecified represents an incomplete state, used for resources that do not
	// ever touch the snapshot serialization layer. This is the default state, but it is not a valid one.
	// snapshot.VerifySnapshot will not allow any resource in the snapshot to have this state.
	ResourceStatusUnspecified MutationStatus = "unspecified"

	// ResourceStatusCreating is the state of resources that have an outstanding Create operation.
	// Once the operation completes, the resource will transition into the "created" state.
	ResourceStatusCreating MutationStatus = "creating"

	// ResourceStatusCreated is the state of resources that have been created successfully.
	ResourceStatusCreated MutationStatus = "created"

	// ResourceStatusUpdating is the state of resources that have an update pending. Once the resource operation
	// completes, the resource will transition into the "updated" state. The inputs of a resource in the
	// "updating" state are the inputs that were used to update the existing resource.
	ResourceStatusUpdating MutationStatus = "updating"

	// ResourceStatusUpdated is the state of resources that have been successfully updated.
	ResourceStatusUpdated MutationStatus = "updated"

	// ResourceStatusDeleting is the state of resources that have a delete operation in-flight.
	// Once the resource operation completes, the resource will be removed from the snapshot.
	ResourceStatusDeleting MutationStatus = "deleting"

	// ResourceStatusPendingDeletion is the state of resources that are slated for deletion, but the delete
	// operation has not yet begin. Resources that have been replaced but not yet deleted are in this state.
	// When the engine decides to delete this resource, it will transition into the "deleting" state, where
	// it will be removed from the snapshot once the deletion completes successfully.
	ResourceStatusPendingDeletion MutationStatus = "pending-deletion"
)

// Live returns whether or not this MutationStatus represents a "live" state. A state is "live" if it
// represents a resource that exists and is not slated for deletion as part of the current plan.
//
// Conceptually, a plan is complete once there are no further modifications to be made and all resources
// are in a live state.
func (rs MutationStatus) Live() bool {
	switch rs {
	case ResourceStatusCreated, ResourceStatusUpdated:
		return true
	}

	return false
}

// State is a structure containing state associated with a resource.  This resource may have been serialized and
// deserialized, or snapshotted from a live graph of resource objects.  The value's state is not, however, associated
// with any runtime objects in memory that may be actively involved in ongoing computations.
type State struct {
	Type         tokens.Type    // the resource's type.
	URN          URN            // the resource's object urn, a human-friendly, unique name for the resource.
	Custom       bool           // true if the resource is custom, managed by a plugin.
	Delete       bool           // true if this resource is pending deletion due to a replacement.
	ID           ID             // the resource's unique ID, assigned by the resource provider or blank if uncreated.
	Inputs       PropertyMap    // the resource's input properties (as specified by the program).
	Outputs      PropertyMap    // the resource's complete output state (as returned by the resource provider).
	Parent       URN            // an optional parent URN that this resource belongs to.
	Protect      bool           // true to "protect" this resource (protected resources cannot be deleted).
	Status       MutationStatus // the status of this resource
	CreatedAt    time.Time      // the time this resource was created
	UpdatedAt    time.Time      // the time this resource was last updated
	Dependencies []URN          // the resource's dependencies
}

// NewState creates a new resource value from existing resource state information.
func NewState(t tokens.Type, urn URN, custom bool, del bool, id ID,
	inputs PropertyMap, outputs PropertyMap, parent URN, protect bool,
	status MutationStatus, createdAt time.Time, updatedAt time.Time, dependencies []URN) *State {
	contract.Assertf(t != "", "type was empty")
	contract.Assertf(custom || id == "", "is custom or had empty ID")
	contract.Assertf(inputs != nil, "inputs was non-nil")
	return &State{
		Type:         t,
		URN:          urn,
		Custom:       custom,
		Delete:       del,
		ID:           id,
		Inputs:       inputs,
		Outputs:      outputs,
		Parent:       parent,
		Protect:      protect,
		Status:       status,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		Dependencies: dependencies,
	}
}

// Clone deeply clones a resource state, producing a new State that is identical to
// the existing one but with no shared references.
func (s *State) Clone() *State {
	var cloned State
	cloned.Type = s.Type
	cloned.URN = s.URN
	cloned.Custom = s.Custom
	cloned.Delete = s.Delete
	cloned.ID = s.ID
	cloned.Inputs = make(PropertyMap)
	for key, value := range s.Inputs {
		cloned.Inputs[key] = value
	}

	cloned.Outputs = make(PropertyMap)
	for key, value := range s.Outputs {
		cloned.Outputs[key] = value
	}

	cloned.Parent = s.Parent
	cloned.Protect = s.Protect
	cloned.Status = s.Status
	cloned.CreatedAt = s.CreatedAt
	cloned.UpdatedAt = s.UpdatedAt
	cloned.Dependencies = append([]URN{}, s.Dependencies...)
	return &cloned
}

func (s *State) CopyIntoWithoutMetadata(other *State) {
	cloned := s.Clone()
	other.Type = cloned.Type
	other.URN = cloned.URN
	other.Custom = cloned.Custom
	other.Delete = cloned.Delete
	other.ID = cloned.ID
	other.Inputs = cloned.Inputs
	other.Outputs = cloned.Outputs
	other.Parent = cloned.Parent
	other.Protect = cloned.Protect
	other.Dependencies = cloned.Dependencies
}

// All returns all resource state, including the inputs and outputs, overlaid in that order.
func (s *State) All() PropertyMap {
	return s.Inputs.Merge(s.Outputs)
}
