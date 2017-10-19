// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Step is a specification for a deployment operation.
type Step interface {
	Op() StepOp                      // the operation performed by this step.
	Plan() *Plan                     // the owning plan.
	Iterator() *PlanIterator         // the current plan iterator.
	Type() tokens.Type               // the type affected by this step.
	Apply() (resource.Status, error) // applies the action that this step represents.
	Skip() error                     // skips past this step (required when iterating a plan).
	Logical() bool                   // true if this step represents a logical operation in the program.
}

// MutatingStep is a step that, when performed, will actually modify/mutate the target stack and its resources.
type MutatingStep interface {
	Step
	URN() resource.URN    // the resource URN (for before and after).
	Old() *resource.State // the state of the resource before performing this step.
	New() *resource.State // the state of the resource after performing this step.
}

// SameStep is a mutating step that does nothing.
type SameStep struct {
	iter *PlanIterator   // the current plan iteration.
	goal SourceGoal      // the goal state for the resource.
	old  *resource.State // the state of the resource before this step.
	new  *resource.State // the state of the resource after this step.
}

var _ MutatingStep = (*SameStep)(nil)

func NewSameStep(iter *PlanIterator, goal SourceGoal, old *resource.State, new *resource.State) Step {
	contract.Assert(goal != nil)
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!old.Delete)
	contract.Assert(new != nil)
	contract.Assert(new.URN != "")
	contract.Assert(new.ID == "")
	contract.Assert(!new.Delete)
	return &SameStep{
		iter: iter,
		goal: goal,
		old:  old,
		new:  new,
	}
}

func (s *SameStep) Op() StepOp              { return OpSame }
func (s *SameStep) Plan() *Plan             { return s.iter.p }
func (s *SameStep) Iterator() *PlanIterator { return s.iter }
func (s *SameStep) Type() tokens.Type       { return s.old.Type }
func (s *SameStep) URN() resource.URN       { return s.old.URN }
func (s *SameStep) Old() *resource.State    { return s.old }
func (s *SameStep) New() *resource.State    { return s.new }
func (s *SameStep) Logical() bool           { return true }

func (s *SameStep) Apply() (resource.Status, error) {
	// Just propagate the ID and output state to the live object and append to the snapshot.
	s.goal.Done(s.old, true, nil)
	s.iter.MarkStateSnapshot(s.old)
	s.iter.AppendStateSnapshot(s.old)
	return resource.StatusOK, nil
}

func (s *SameStep) Skip() error {
	// In the case of a same, both ID and outputs are identical.
	s.goal.Done(s.old, true, nil)
	return nil
}

// CreateStep is a mutating step that creates an entirely new resource.
type CreateStep struct {
	iter      *PlanIterator          // the current plan iteration.
	goal      SourceGoal             // the goal state for the resource.
	old       *resource.State        // the state of the existing resource (only for replacements).
	new       *resource.State        // the state of the resource after this step.
	keys      []resource.PropertyKey // the keys causing replacement (only for replacements).
	replacing bool                   // true if this is a create due to a replacement.
}

var _ MutatingStep = (*CreateStep)(nil)

func NewCreateStep(iter *PlanIterator, goal SourceGoal, new *resource.State) Step {
	contract.Assert(goal != nil)
	contract.Assert(new != nil)
	contract.Assert(new.URN != "")
	contract.Assert(new.ID == "")
	contract.Assert(!new.Delete)
	return &CreateStep{
		iter: iter,
		goal: goal,
		new:  new,
	}
}

func NewCreateReplacementStep(iter *PlanIterator, goal SourceGoal,
	old *resource.State, new *resource.State, keys []resource.PropertyKey) Step {
	contract.Assert(goal != nil)
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!old.Delete)
	contract.Assert(new != nil)
	contract.Assert(new.URN != "")
	contract.Assert(new.ID == "")
	contract.Assert(!new.Delete)
	contract.Assert(old.Type == new.Type)
	return &CreateStep{
		iter:      iter,
		goal:      goal,
		old:       old,
		new:       new,
		keys:      keys,
		replacing: true,
	}
}

func (s *CreateStep) Op() StepOp {
	if s.replacing {
		return OpCreateReplacement
	}
	return OpCreate
}
func (s *CreateStep) Plan() *Plan                  { return s.iter.p }
func (s *CreateStep) Iterator() *PlanIterator      { return s.iter }
func (s *CreateStep) Type() tokens.Type            { return s.new.Type }
func (s *CreateStep) URN() resource.URN            { return s.new.URN }
func (s *CreateStep) Old() *resource.State         { return s.old }
func (s *CreateStep) New() *resource.State         { return s.new }
func (s *CreateStep) Keys() []resource.PropertyKey { return s.keys }
func (s *CreateStep) Logical() bool                { return !s.replacing }

func (s *CreateStep) Apply() (resource.Status, error) {
	if s.new.Custom {
		// Invoke the Create RPC function for this provider:
		prov, err := getProvider(s)
		if err != nil {
			return resource.StatusOK, err
		}
		id, outs, rst, err := prov.Create(s.URN(), s.new.AllInputs())
		if err != nil {
			return rst, err
		}
		contract.Assert(id != "")

		// Copy any of the default and output properties on the live object state.
		s.new.ID = id
		s.new.Outputs = outs
	}

	// Mark the old resource as pending deletion if necessary.
	if s.replacing {
		s.old.Delete = true
	}

	// And finish the overall operation.
	s.goal.Done(s.new, false, nil)
	s.iter.AppendStateSnapshot(s.new)
	return resource.StatusOK, nil
}

func (s *CreateStep) Skip() error {
	// In the case of a create, we don't know the ID or output properties.  But we do know the defaults and URN.
	s.goal.Done(s.new, false, nil)
	return nil
}

// DeleteStep is a mutating step that deletes an existing resource.
type DeleteStep struct {
	iter      *PlanIterator   // the current plan iteration.
	old       *resource.State // the state of the existing resource.
	replacing bool            // true if part of a replacement.
}

var _ MutatingStep = (*DeleteStep)(nil)

func NewDeleteStep(iter *PlanIterator, old *resource.State, replacing bool) Step {
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!replacing || old.Delete)
	return &DeleteStep{
		iter:      iter,
		old:       old,
		replacing: replacing,
	}
}

func (s *DeleteStep) Op() StepOp {
	if s.replacing {
		return OpDeleteReplaced
	}
	return OpDelete
}
func (s *DeleteStep) Plan() *Plan             { return s.iter.p }
func (s *DeleteStep) Iterator() *PlanIterator { return s.iter }
func (s *DeleteStep) Type() tokens.Type       { return s.old.Type }
func (s *DeleteStep) URN() resource.URN       { return s.old.URN }
func (s *DeleteStep) Old() *resource.State    { return s.old }
func (s *DeleteStep) New() *resource.State    { return nil }
func (s *DeleteStep) Logical() bool           { return !s.replacing }

func (s *DeleteStep) Apply() (resource.Status, error) {
	if s.old.Custom {
		// Invoke the Delete RPC function for this provider:
		prov, err := getProvider(s)
		if err != nil {
			return resource.StatusOK, err
		}
		if rst, err := prov.Delete(s.URN(), s.old.ID, s.old.All()); err != nil {
			return rst, err
		}
	}
	s.iter.MarkStateSnapshot(s.old)
	return resource.StatusOK, nil
}

func (s *DeleteStep) Skip() error {
	// In the case of a deletion, there is no state to propagate: the new object doesn't even exist.
	return nil
}

// UpdateStep is a mutating step that updates an existing resource's state.
type UpdateStep struct {
	iter    *PlanIterator          // the current plan iteration.
	goal    SourceGoal             // the goal state for the resource.
	old     *resource.State        // the state of the existing resource.
	new     *resource.State        // the newly computed state of the resource after updating.
	stables []resource.PropertyKey // an optional list of properties that won't change during this update.
}

var _ MutatingStep = (*UpdateStep)(nil)

func NewUpdateStep(iter *PlanIterator, goal SourceGoal, old *resource.State,
	new *resource.State, stables []resource.PropertyKey) Step {
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!old.Delete)
	contract.Assert(new != nil)
	contract.Assert(new.URN != "")
	contract.Assert(new.ID == "")
	contract.Assert(!new.Delete)
	contract.Assert(old.Type == new.Type)
	return &UpdateStep{
		iter:    iter,
		goal:    goal,
		old:     old,
		new:     new,
		stables: stables,
	}
}

func (s *UpdateStep) Op() StepOp              { return OpUpdate }
func (s *UpdateStep) Plan() *Plan             { return s.iter.p }
func (s *UpdateStep) Iterator() *PlanIterator { return s.iter }
func (s *UpdateStep) Type() tokens.Type       { return s.old.Type }
func (s *UpdateStep) URN() resource.URN       { return s.old.URN }
func (s *UpdateStep) Old() *resource.State    { return s.old }
func (s *UpdateStep) New() *resource.State    { return s.new }
func (s *UpdateStep) Logical() bool           { return true }

func (s *UpdateStep) Apply() (resource.Status, error) {
	if s.new.Custom {
		// Invoke the Update RPC function for this provider:
		prov, err := getProvider(s)
		if err != nil {
			return resource.StatusOK, err
		}
		outs, rst, upderr := prov.Update(s.URN(), s.old.ID, s.old.AllInputs(), s.new.AllInputs())
		if upderr != nil {
			return rst, upderr
		}

		// Now copy any output state back in case the update triggered cascading updates to other properties.
		s.new.ID = s.old.ID
		s.new.Outputs = outs
	}

	// Finally, mark this operation as complete.
	s.goal.Done(s.new, false, s.stables)
	s.iter.MarkStateSnapshot(s.old)
	s.iter.AppendStateSnapshot(s.new)
	return resource.StatusOK, nil
}

func (s *UpdateStep) Skip() error {
	// In the case of an update, the URN, defaults, and ID are the same, however, the outputs remain unknown.
	s.new.ID = s.old.ID
	s.goal.Done(s.new, false, s.stables)
	return nil
}

// ReplaceStep is a logical step indicating a resource will be replaced.  This is comprised of three physical steps:
// a creation of the new resource, any number of intervening updates of dependents to the new resource, and then
// a deletion of the now-replaced old resource.  This logical step is primarily here for tools and visualization.
type ReplaceStep struct {
	iter *PlanIterator          // the current plan iteration.
	old  *resource.State        // the state of the existing resource.
	new  *resource.State        // the new state snapshot.
	keys []resource.PropertyKey // the keys causing replacement.
}

func NewReplaceStep(iter *PlanIterator, old *resource.State, new *resource.State, keys []resource.PropertyKey) Step {
	contract.Assert(old != nil)
	contract.Assert(old.URN != "")
	contract.Assert(old.ID != "" || !old.Custom)
	contract.Assert(!old.Delete)
	contract.Assert(new != nil)
	contract.Assert(new.URN != "")
	contract.Assert(new.ID == "")
	contract.Assert(!new.Delete)
	return &ReplaceStep{
		iter: iter,
		old:  old,
		new:  new,
		keys: keys,
	}
}

func (s *ReplaceStep) Op() StepOp                   { return OpReplace }
func (s *ReplaceStep) Plan() *Plan                  { return s.iter.p }
func (s *ReplaceStep) Iterator() *PlanIterator      { return s.iter }
func (s *ReplaceStep) Type() tokens.Type            { return s.old.Type }
func (s *ReplaceStep) URN() resource.URN            { return s.old.URN }
func (s *ReplaceStep) Old() *resource.State         { return s.old }
func (s *ReplaceStep) New() *resource.State         { return s.new }
func (s *ReplaceStep) Keys() []resource.PropertyKey { return s.keys }
func (s *ReplaceStep) Logical() bool                { return true }

func (s *ReplaceStep) Apply() (resource.Status, error) {
	// We should have marked the old resource for deletion in the CreateReplacement step.
	contract.Assert(s.old.Delete)
	return resource.StatusOK, nil
}

func (s *ReplaceStep) Skip() error {
	return nil
}

// getProvider fetches the provider for the given step.
func getProvider(s Step) (plugin.Provider, error) {
	return s.Plan().Provider(s.Type().Package())
}

// StepOp represents the kind of operation performed by a step.  It evaluates to its string label.
type StepOp string

const (
	OpSame              StepOp = "same"               // nothing to do.
	OpCreate            StepOp = "create"             // creating a new resource.
	OpUpdate            StepOp = "modify"             // modifying an existing resource.
	OpDelete            StepOp = "delete"             // deleting an existing resource.
	OpReplace           StepOp = "replace"            // replacing a resource with a new one.
	OpCreateReplacement StepOp = "create-replacement" // creating a new resource for a replacement.
	OpDeleteReplaced    StepOp = "delete-replaced"    // deleting an existing resource after replacement.
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
}

// Color returns a suggested color for lines of this op type.
func (op StepOp) Color() string {
	switch op {
	case OpSame:
		return ""
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
	default:
		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
}

// Prefix returns a suggested prefix for lines of this op type.
func (op StepOp) Prefix() string {
	switch op {
	case OpSame:
		return op.Color() + "  "
	case OpCreate:
		return op.Color() + "+ "
	case OpDelete:
		return op.Color() + "- "
	case OpUpdate:
		return op.Color() + "~ "
	case OpReplace:
		return op.Color() + "+-"
	case OpCreateReplacement:
		return op.Color() + " +"
	case OpDeleteReplaced:
		return op.Color() + " -"
	default:
		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
}

func (op StepOp) PastTense() string {
	switch op {
	case OpSame, OpCreate, OpDelete, OpReplace, OpCreateReplacement, OpDeleteReplaced:
		return string(op) + "d"
	case OpUpdate:
		return "modified"
	default:
		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
}

// Suffix returns a suggested suffix for lines of this op type.
func (op StepOp) Suffix() string {
	if op == OpCreateReplacement || op == OpUpdate || op == OpReplace {
		return colors.Reset // updates and replacements colorize individual lines; get has none
	}
	return ""
}
