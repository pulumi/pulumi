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

package deploy

import (
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// StepCompleteFunc is the type of functions returned from Step.Apply. These functions are to be called
// when the engine has fully retired a step.
type StepCompleteFunc func()

// Step is a specification for a deployment operation.
type Step interface {
	// Apply applies or previews this step. It returns the status of the resource after the step application,
	// a function to call to signal that this step has fully completed, and an error, if one occurred while applying
	// the step.
	//
	// The returned StepCompleteFunc, if not nil, must be called after committing the results of this step into
	// the state of the deployment.
	Apply(preview bool) (resource.Status, StepCompleteFunc, error) // applies or previews this step.

	Op() StepOp           // the operation performed by this step.
	URN() resource.URN    // the resource URN (for before and after).
	Type() tokens.Type    // the type affected by this step.
	Provider() string     // the provider reference for this step.
	Old() *resource.State // the state of the resource before performing this step.
	New() *resource.State // the state of the resource after performing this step.
	Res() *resource.State // the latest state for the resource that is known (worst case, old).
	Logical() bool        // true if this step represents a logical operation in the program.
	Plan() *Plan          // the owning plan.
}

// SameStep is a mutating step that does nothing.
type SameStep struct {
	plan *Plan                 // the current plan.
	reg  RegisterResourceEvent // the registration intent to convey a URN back to.
	old  *resource.State       // the state of the resource before this step.
	new  *resource.State       // the state of the resource after this step.
}

var _ Step = (*SameStep)(nil)

func NewSameStep(plan *Plan, reg RegisterResourceEvent, old *resource.State, new *resource.State) Step {
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!old.Custom || old.Provider != "" || providers.IsProviderType(old.Type))
	contract.Assert(!old.Delete)
	contract.Assert(new != nil)
	contract.Assert(new.URN != "")
	contract.Assert(new.ID == "")
	contract.Assert(!new.Delete)
	return &SameStep{
		plan: plan,
		reg:  reg,
		old:  old,
		new:  new,
	}
}

func (s *SameStep) Op() StepOp           { return OpSame }
func (s *SameStep) Plan() *Plan          { return s.plan }
func (s *SameStep) Type() tokens.Type    { return s.old.Type }
func (s *SameStep) Provider() string     { return s.old.Provider }
func (s *SameStep) URN() resource.URN    { return s.old.URN }
func (s *SameStep) Old() *resource.State { return s.old }
func (s *SameStep) New() *resource.State { return s.new }
func (s *SameStep) Res() *resource.State { return s.new }
func (s *SameStep) Logical() bool        { return true }

func (s *SameStep) Apply(preview bool) (resource.Status, StepCompleteFunc, error) {
	// Retain the URN, ID, and outputs:
	s.new.URN = s.old.URN
	s.new.ID = s.old.ID
	s.new.Outputs = s.old.Outputs
	complete := func() { s.reg.Done(&RegisterResult{State: s.new, Stable: true}) }
	return resource.StatusOK, complete, nil
}

// CreateStep is a mutating step that creates an entirely new resource.
type CreateStep struct {
	plan          *Plan                  // the current plan.
	reg           RegisterResourceEvent  // the registration intent to convey a URN back to.
	old           *resource.State        // the state of the existing resource (only for replacements).
	new           *resource.State        // the state of the resource after this step.
	keys          []resource.PropertyKey // the keys causing replacement (only for replacements).
	replacing     bool                   // true if this is a create due to a replacement.
	pendingDelete bool                   // true if this replacement should create a pending delete.
}

var _ Step = (*CreateStep)(nil)

func NewCreateStep(plan *Plan, reg RegisterResourceEvent, new *resource.State) Step {
	contract.Assert(reg != nil)
	contract.Assert(new != nil)
	contract.Assert(new.URN != "")
	contract.Assert(new.ID == "")
	contract.Assert(!new.Custom || new.Provider != "" || providers.IsProviderType(new.Type))
	contract.Assert(!new.Delete)
	contract.Assert(!new.External)
	return &CreateStep{
		plan: plan,
		reg:  reg,
		new:  new,
	}
}

func NewCreateReplacementStep(plan *Plan, reg RegisterResourceEvent,
	old *resource.State, new *resource.State, keys []resource.PropertyKey, pendingDelete bool) Step {
	contract.Assert(reg != nil)
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!old.Delete)
	contract.Assert(new != nil)
	contract.Assert(new.URN != "")
	contract.Assert(new.ID == "")
	contract.Assert(!new.Custom || new.Provider != "" || providers.IsProviderType(new.Type))
	contract.Assert(!new.Delete)
	contract.Assert(old.Type == new.Type)
	contract.Assert(!new.External)
	return &CreateStep{
		plan:          plan,
		reg:           reg,
		old:           old,
		new:           new,
		keys:          keys,
		replacing:     true,
		pendingDelete: pendingDelete,
	}
}

func (s *CreateStep) Op() StepOp {
	if s.replacing {
		return OpCreateReplacement
	}
	return OpCreate
}
func (s *CreateStep) Plan() *Plan                  { return s.plan }
func (s *CreateStep) Type() tokens.Type            { return s.new.Type }
func (s *CreateStep) Provider() string             { return s.new.Provider }
func (s *CreateStep) URN() resource.URN            { return s.new.URN }
func (s *CreateStep) Old() *resource.State         { return s.old }
func (s *CreateStep) New() *resource.State         { return s.new }
func (s *CreateStep) Res() *resource.State         { return s.new }
func (s *CreateStep) Keys() []resource.PropertyKey { return s.keys }
func (s *CreateStep) Logical() bool                { return !s.replacing }

func (s *CreateStep) Apply(preview bool) (resource.Status, StepCompleteFunc, error) {
	var resourceError error
	resourceStatus := resource.StatusOK
	if !preview {
		if s.new.Custom {
			// Invoke the Create RPC function for this provider:
			prov, err := getProvider(s)
			if err != nil {
				return resource.StatusOK, nil, err
			}
			id, outs, rst, err := prov.Create(s.URN(), s.new.Inputs)
			if err != nil {
				if rst != resource.StatusPartialFailure {
					return rst, nil, err
				}

				resourceError = err
				resourceStatus = rst

				if initErr, isInitErr := err.(*plugin.InitError); isInitErr {
					s.new.InitErrors = initErr.Reasons
				}
			}

			contract.Assert(id != "")

			// Copy any of the default and output properties on the live object state.
			s.new.ID = id
			s.new.Outputs = outs
		}
	}

	// Mark the old resource as pending deletion if necessary.
	if s.replacing && s.pendingDelete {
		s.old.Delete = true
	}

	complete := func() { s.reg.Done(&RegisterResult{State: s.new}) }
	if resourceError == nil {
		return resourceStatus, complete, nil
	}
	return resourceStatus, complete, resourceError
}

// DeleteStep is a mutating step that deletes an existing resource. If `old` is marked "External",
// DeleteStep is a no-op.
type DeleteStep struct {
	plan      *Plan           // the current plan.
	old       *resource.State // the state of the existing resource.
	replacing bool            // true if part of a replacement.
}

var _ Step = (*DeleteStep)(nil)

func NewDeleteStep(plan *Plan, old *resource.State) Step {
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!old.Custom || old.Provider != "" || providers.IsProviderType(old.Type))
	contract.Assert(!old.Delete)
	return &DeleteStep{
		plan: plan,
		old:  old,
	}
}

func NewDeleteReplacementStep(plan *Plan, old *resource.State, pendingDelete bool) Step {
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!old.Custom || old.Provider != "" || providers.IsProviderType(old.Type))
	contract.Assert(!pendingDelete || old.Delete)
	return &DeleteStep{
		plan:      plan,
		old:       old,
		replacing: true,
	}
}

func (s *DeleteStep) Op() StepOp {
	if s.replacing {
		return OpDeleteReplaced
	}
	return OpDelete
}
func (s *DeleteStep) Plan() *Plan          { return s.plan }
func (s *DeleteStep) Type() tokens.Type    { return s.old.Type }
func (s *DeleteStep) Provider() string     { return s.old.Provider }
func (s *DeleteStep) URN() resource.URN    { return s.old.URN }
func (s *DeleteStep) Old() *resource.State { return s.old }
func (s *DeleteStep) New() *resource.State { return nil }
func (s *DeleteStep) Res() *resource.State { return s.old }
func (s *DeleteStep) Logical() bool        { return !s.replacing }

func (s *DeleteStep) Apply(preview bool) (resource.Status, StepCompleteFunc, error) {
	// Refuse to delete protected resources.
	if s.old.Protect {
		return resource.StatusOK, nil,
			errors.Errorf("refusing to delete protected resource '%s'", s.old.URN)
	}

	// Deleting an External resource is a no-op, since Pulumi does not own the lifecycle.
	if !preview && !s.old.External {
		if s.old.Custom {
			// Invoke the Delete RPC function for this provider:
			prov, err := getProvider(s)
			if err != nil {
				return resource.StatusOK, nil, err
			}
			if rst, err := prov.Delete(s.URN(), s.old.ID, s.old.All()); err != nil {
				return rst, nil, err
			}
		}
	}

	return resource.StatusOK, func() {}, nil
}

// UpdateStep is a mutating step that updates an existing resource's state.
type UpdateStep struct {
	plan    *Plan                  // the current plan.
	reg     RegisterResourceEvent  // the registration intent to convey a URN back to.
	old     *resource.State        // the state of the existing resource.
	new     *resource.State        // the newly computed state of the resource after updating.
	stables []resource.PropertyKey // an optional list of properties that won't change during this update.
}

var _ Step = (*UpdateStep)(nil)

func NewUpdateStep(plan *Plan, reg RegisterResourceEvent, old *resource.State,
	new *resource.State, stables []resource.PropertyKey) Step {
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!old.Custom || old.Provider != "" || providers.IsProviderType(old.Type))
	contract.Assert(!old.Delete)
	contract.Assert(new != nil)
	contract.Assert(new.URN != "")
	contract.Assert(new.ID == "")
	contract.Assert(!new.Delete)
	contract.Assert(old.Type == new.Type)
	contract.Assert(!new.External)
	contract.Assert(!old.External)
	return &UpdateStep{
		plan:    plan,
		reg:     reg,
		old:     old,
		new:     new,
		stables: stables,
	}
}

func (s *UpdateStep) Op() StepOp           { return OpUpdate }
func (s *UpdateStep) Plan() *Plan          { return s.plan }
func (s *UpdateStep) Type() tokens.Type    { return s.old.Type }
func (s *UpdateStep) Provider() string     { return s.old.Provider }
func (s *UpdateStep) URN() resource.URN    { return s.old.URN }
func (s *UpdateStep) Old() *resource.State { return s.old }
func (s *UpdateStep) New() *resource.State { return s.new }
func (s *UpdateStep) Res() *resource.State { return s.new }
func (s *UpdateStep) Logical() bool        { return true }

func (s *UpdateStep) Apply(preview bool) (resource.Status, StepCompleteFunc, error) {
	// Always propagate the URN and ID, even in previews and refreshes.
	s.new.URN = s.old.URN
	s.new.ID = s.old.ID

	var resourceError error
	resourceStatus := resource.StatusOK
	if !preview {
		if s.new.Custom {
			// Invoke the Update RPC function for this provider:
			prov, err := getProvider(s)
			if err != nil {
				return resource.StatusOK, nil, err
			}

			// Update to the combination of the old "all" state (including outputs), but overwritten with new inputs.
			outs, rst, upderr := prov.Update(s.URN(), s.old.ID, s.old.All(), s.new.Inputs)
			if upderr != nil {
				if rst != resource.StatusPartialFailure {
					return rst, nil, upderr
				}

				resourceError = upderr
				resourceStatus = rst

				if initErr, isInitErr := upderr.(*plugin.InitError); isInitErr {
					s.new.InitErrors = initErr.Reasons
				}
			}

			// Now copy any output state back in case the update triggered cascading updates to other properties.
			s.new.Outputs = outs
		}
	}

	// Finally, mark this operation as complete.
	complete := func() { s.reg.Done(&RegisterResult{State: s.new, Stables: s.stables}) }
	if resourceError == nil {
		return resourceStatus, complete, nil
	}
	return resourceStatus, complete, resourceError
}

// ReplaceStep is a logical step indicating a resource will be replaced.  This is comprised of three physical steps:
// a creation of the new resource, any number of intervening updates of dependents to the new resource, and then
// a deletion of the now-replaced old resource.  This logical step is primarily here for tools and visualization.
type ReplaceStep struct {
	plan          *Plan                  // the current plan.
	old           *resource.State        // the state of the existing resource.
	new           *resource.State        // the new state snapshot.
	keys          []resource.PropertyKey // the keys causing replacement.
	pendingDelete bool                   // true if a pending deletion should happen.
}

var _ Step = (*ReplaceStep)(nil)

func NewReplaceStep(plan *Plan, old *resource.State, new *resource.State,
	keys []resource.PropertyKey, pendingDelete bool) Step {
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!old.Delete)
	contract.Assert(new != nil)
	contract.Assert(new.URN != "")
	// contract.Assert(new.ID == "")
	contract.Assert(!new.Delete)
	return &ReplaceStep{
		plan:          plan,
		old:           old,
		new:           new,
		keys:          keys,
		pendingDelete: pendingDelete,
	}
}

func (s *ReplaceStep) Op() StepOp                   { return OpReplace }
func (s *ReplaceStep) Plan() *Plan                  { return s.plan }
func (s *ReplaceStep) Type() tokens.Type            { return s.old.Type }
func (s *ReplaceStep) Provider() string             { return s.old.Provider }
func (s *ReplaceStep) URN() resource.URN            { return s.old.URN }
func (s *ReplaceStep) Old() *resource.State         { return s.old }
func (s *ReplaceStep) New() *resource.State         { return s.new }
func (s *ReplaceStep) Res() *resource.State         { return s.new }
func (s *ReplaceStep) Keys() []resource.PropertyKey { return s.keys }
func (s *ReplaceStep) Logical() bool                { return true }

func (s *ReplaceStep) Apply(preview bool) (resource.Status, StepCompleteFunc, error) {
	// If this is a pending delete, we should have marked the old resource for deletion in the CreateReplacement step.
	contract.Assert(!s.pendingDelete || s.old.Delete)
	return resource.StatusOK, func() {}, nil
}

// ReadStep is a step indicating that an existing resources will be "read" and projected into the Pulumi object
// model. Resources that are read are marked with the "External" bit which indicates to the engine that it does
// not own this resource's lifeycle.
//
// A resource with a given URN can transition freely between an "external" state and a non-external state. If
// a URN that was previously marked "External" (i.e. was the target of a ReadStep in a previous plan) is the
// target of a RegisterResource in the next plan, a CreateReplacement step will be issued to indicate the transition
// from external to owned. If a URN that was previously not marked "External" is the target of a ReadResource in the
// next plan, a ReadReplacement step will be issued to indicate the transition from owned to external.
type ReadStep struct {
	plan      *Plan             // the plan that produced this read
	event     ReadResourceEvent // the event that should be signaled upon completion
	old       *resource.State   // the old resource state, if one exists for this urn
	new       *resource.State   // the new resource state, to be used to query the provider
	replacing bool              // whether or not the new resource is replacing the old resource
}

// NewReadStep creates a new Read step.
func NewReadStep(plan *Plan, event ReadResourceEvent, old *resource.State, new *resource.State) Step {
	contract.Assert(new != nil)
	contract.Assertf(new.External, "target of Read step must be marked External")
	contract.Assertf(new.Custom, "target of Read step must be Custom")

	// If Old was given, it's either an external resource or its ID is equal to the
	// ID that we are preparing to read.
	if old != nil {
		contract.Assert(old.ID == new.ID || old.External)
	}

	return &ReadStep{
		plan:      plan,
		event:     event,
		old:       old,
		new:       new,
		replacing: false,
	}
}

// NewReadReplacementStep creates a new Read step with the `replacing` flag set. When executed,
// it will pend deletion of the "old" resource, which must not be an external resource.
func NewReadReplacementStep(plan *Plan, event ReadResourceEvent, old *resource.State, new *resource.State) Step {
	contract.Assert(new != nil)
	contract.Assertf(new.External, "target of ReadReplacement step must be marked External")
	contract.Assertf(new.Custom, "target of ReadReplacement step must be Custom")
	contract.Assert(old != nil)
	contract.Assertf(!old.External, "old target of ReadReplacement step must not be External")
	return &ReadStep{
		plan:      plan,
		event:     event,
		old:       old,
		new:       new,
		replacing: true,
	}
}

func (s *ReadStep) Op() StepOp {
	if s.replacing {
		return OpReadReplacement
	}

	return OpRead
}

func (s *ReadStep) Plan() *Plan          { return s.plan }
func (s *ReadStep) Type() tokens.Type    { return s.new.Type }
func (s *ReadStep) Provider() string     { return s.new.Provider }
func (s *ReadStep) URN() resource.URN    { return s.new.URN }
func (s *ReadStep) Old() *resource.State { return s.old }
func (s *ReadStep) New() *resource.State { return s.new }
func (s *ReadStep) Res() *resource.State { return s.new }
func (s *ReadStep) Logical() bool        { return !s.replacing }

func (s *ReadStep) Apply(preview bool) (resource.Status, StepCompleteFunc, error) {
	urn := s.new.URN
	id := s.new.ID

	var resourceError error
	resourceStatus := resource.StatusOK
	// Unlike most steps, Read steps run during previews. The only time
	// we can't run is if the ID we are given is unknown.
	if id == "" || id == plugin.UnknownStringValue {
		s.new.Outputs = resource.PropertyMap{}
	} else {
		prov, err := getProvider(s)
		if err != nil {
			return resource.StatusOK, nil, err
		}

		result, rst, err := prov.Read(urn, id, s.new.Inputs)
		if err != nil {
			if rst != resource.StatusPartialFailure {
				return rst, nil, err
			}

			resourceError = err
			resourceStatus = rst

			if initErr, isInitErr := err.(*plugin.InitError); isInitErr {
				s.new.InitErrors = initErr.Reasons
			}
		}

		s.new.Outputs = result
	}

	// If we were asked to replace an existing, non-External resource, pend the
	// deletion here.
	if s.replacing {
		s.old.Delete = true
	}

	complete := func() { s.event.Done(&ReadResult{State: s.new}) }
	if resourceError == nil {
		return resourceStatus, complete, nil
	}
	return resourceStatus, complete, resourceError
}

// RefreshStep is a step used to track the progress of a refresh operation. A refresh operation updates the an existing
// resource by reading its current state from its provider plugin. These steps are not issued by the step generator;
// instead, they are issued by the plan executor as the optional first step in plan execution.
type RefreshStep struct {
	plan *Plan           // the plan that produced this refresh
	old  *resource.State // the old resource state, if one exists for this urn
	new  *resource.State // the new resource state, to be used to query the provider
	done chan<- bool     // the channel to use to signal completion, if any
}

// NewRefreshStep creates a new Refresh step.
func NewRefreshStep(plan *Plan, old *resource.State, done chan<- bool) Step {
	contract.Assert(old != nil)

	// NOTE: we set the new state to the old state by default so that we don't interpret step failures as deletes.
	return &RefreshStep{
		plan: plan,
		old:  old,
		new:  old,
		done: done,
	}
}

func (s *RefreshStep) Op() StepOp           { return OpRefresh }
func (s *RefreshStep) Plan() *Plan          { return s.plan }
func (s *RefreshStep) Type() tokens.Type    { return s.old.Type }
func (s *RefreshStep) Provider() string     { return s.old.Provider }
func (s *RefreshStep) URN() resource.URN    { return s.old.URN }
func (s *RefreshStep) Old() *resource.State { return s.old }
func (s *RefreshStep) New() *resource.State { return s.new }
func (s *RefreshStep) Res() *resource.State { return s.old }
func (s *RefreshStep) Logical() bool        { return false }

// ResultOp returns the operation that corresponds to the change to this resource after reading its current state, if
// any.
func (s *RefreshStep) ResultOp() StepOp {
	if s.new == nil {
		return OpDelete
	}
	if s.new == s.old || s.old.Outputs.Diff(s.new.Outputs) == nil {
		return OpSame
	}
	return OpUpdate
}

func (s *RefreshStep) Apply(preview bool) (resource.Status, StepCompleteFunc, error) {
	var complete func()
	if s.done != nil {
		complete = func() { close(s.done) }
	}

	// Component and provider resources never change with a refresh; just return the current state.
	if !s.old.Custom || providers.IsProviderType(s.old.Type) {
		return resource.StatusOK, complete, nil
	}

	// For a custom resource, fetch the resource's provider and read the resource's current state.
	prov, err := getProvider(s)
	if err != nil {
		return resource.StatusOK, nil, err
	}

	var initErrors []string
	refreshed, rst, err := prov.Read(s.old.URN, s.old.ID, s.old.Outputs)
	if err != nil {
		if rst != resource.StatusPartialFailure {
			return rst, nil, err
		}
		if initErr, isInitErr := err.(*plugin.InitError); isInitErr {
			// We clear error in this case because we do not want the refresh to fail in the face of initialization
			// errors.
			initErrors = initErr.Reasons
		}
	}

	if refreshed != nil {
		s.new = resource.NewState(s.old.Type, s.old.URN, s.old.Custom, s.old.Delete, s.old.ID, s.old.Inputs, refreshed,
			s.old.Parent, s.old.Protect, s.old.External, s.old.Dependencies, initErrors, s.old.Provider)
	} else {
		s.new = nil
	}

	return rst, complete, err
}

// StepOp represents the kind of operation performed by a step.  It evaluates to its string label.
type StepOp string

const (
	OpSame              StepOp = "same"               // nothing to do.
	OpCreate            StepOp = "create"             // creating a new resource.
	OpUpdate            StepOp = "update"             // updating an existing resource.
	OpDelete            StepOp = "delete"             // deleting an existing resource.
	OpReplace           StepOp = "replace"            // replacing a resource with a new one.
	OpCreateReplacement StepOp = "create-replacement" // creating a new resource for a replacement.
	OpDeleteReplaced    StepOp = "delete-replaced"    // deleting an existing resource after replacement.
	OpRead              StepOp = "read"               // reading an existing resource.
	OpReadReplacement   StepOp = "read-replacement"   // reading an existing resource for a replacement.
	OpRefresh           StepOp = "refresh"            // refreshing an existing resource.
)

// StepOps contains the full set of step operation types.
var StepOps = []StepOp{
	OpSame,
	OpCreate,
	OpUpdate,
	OpDelete,
	OpReplace,
	OpCreateReplacement,
	OpDeleteReplaced,
	OpRead,
	OpReadReplacement,
	OpRefresh,
}

// Color returns a suggested color for lines of this op type.
func (op StepOp) Color() string {
	switch op {
	case OpSame:
		return colors.SpecUnimportant
	case OpCreate:
		return colors.SpecCreate
	case OpDelete:
		return colors.SpecDelete
	case OpUpdate:
		return colors.SpecUpdate
	case OpReplace:
		return colors.SpecReplace
	case OpCreateReplacement:
		return colors.SpecCreateReplacement
	case OpDeleteReplaced:
		return colors.SpecDeleteReplaced
	case OpRead:
		return colors.SpecCreate
	case OpReadReplacement:
		return colors.SpecReplace
	case OpRefresh:
		return colors.SpecUpdate
	default:
		contract.Failf("Unrecognized resource step op: '%v'", op)
		return ""
	}
}

// Prefix returns a suggested prefix for lines of this op type.
func (op StepOp) Prefix() string {
	return op.Color() + op.RawPrefix()
}

// RawPrefix returns the uncolorized prefix text.
func (op StepOp) RawPrefix() string {
	switch op {
	case OpSame:
		return "* "
	case OpCreate:
		return "+ "
	case OpDelete:
		return "- "
	case OpUpdate:
		return "~ "
	case OpReplace:
		return "+-"
	case OpCreateReplacement:
		return "++"
	case OpDeleteReplaced:
		return "--"
	case OpRead:
		return ">-"
	case OpReadReplacement:
		return ">~"
	case OpRefresh:
		return "~ "
	default:
		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
}

func (op StepOp) PastTense() string {
	switch op {
	case OpSame, OpCreate, OpDelete, OpReplace, OpCreateReplacement, OpDeleteReplaced, OpUpdate, OpReadReplacement:
		return string(op) + "d"
	case OpRefresh:
		return "refreshed"
	case OpRead:
		return "read"
	default:
		contract.Failf("Unexpected resource step op: %v", op)
		return ""
	}
}

// Suffix returns a suggested suffix for lines of this op type.
func (op StepOp) Suffix() string {
	if op == OpCreateReplacement || op == OpUpdate || op == OpReplace || op == OpReadReplacement || op == OpRefresh {
		return colors.Reset // updates and replacements colorize individual lines; get has none
	}
	return ""
}

// getProvider fetches the provider for the given step.
func getProvider(s Step) (plugin.Provider, error) {
	if providers.IsProviderType(s.Type()) {
		return s.Plan().providers, nil
	}
	ref, err := providers.ParseReference(s.Provider())
	if err != nil {
		return nil, errors.Errorf("bad provider reference '%v' for resource %v: %v", s.Provider(), s.URN(), err)
	}
	provider, ok := s.Plan().GetProvider(ref)
	if !ok {
		return nil, errors.Errorf("unknown provider '%v' for resource %v", s.Provider(), s.URN())
	}
	return provider, nil
}
