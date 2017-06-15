// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
	"github.com/pulumi/lumi/pkg/diag/colors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/plugin"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Step is a specification for a deployment operation.
type Step struct {
	iter    *PlanIterator          // the current plan iteration.
	op      StepOp                 // the operation that will be performed.
	urn     resource.URN           // the resource URN (for before and after).
	old     *resource.State        // the state of the resource before this step.
	new     *resource.Object       // the state of the resource after this step.
	inputs  resource.PropertyMap   // the input properties to use during the operation.
	outputs resource.PropertyMap   // the output properties calculated after the operation.
	reasons []resource.PropertyKey // the reasons for replacement, if applicable.
}

func NewSameStep(iter *PlanIterator, old *resource.State, new *resource.Object, inputs resource.PropertyMap) *Step {
	contract.Assert(resource.HasURN(old))
	contract.Assert(!resource.HasURN(new))
	return &Step{iter: iter, op: OpSame, urn: old.URN(), old: old, new: new, inputs: inputs}
}

func NewCreateStep(iter *PlanIterator, urn resource.URN, new *resource.Object, inputs resource.PropertyMap) *Step {
	contract.Assert(!resource.HasURN(new))
	return &Step{iter: iter, op: OpCreate, urn: urn, new: new, inputs: inputs}
}

func NewDeleteStep(iter *PlanIterator, old *resource.State) *Step {
	contract.Assert(resource.HasURN(old))
	return &Step{iter: iter, op: OpDelete, urn: old.URN(), old: old}
}

func NewUpdateStep(iter *PlanIterator, old *resource.State,
	new *resource.Object, inputs resource.PropertyMap) *Step {
	contract.Assert(resource.HasURN(old))
	contract.Assert(!resource.HasURN(new))
	return &Step{iter: iter, op: OpUpdate, urn: old.URN(), old: old, new: new, inputs: inputs}
}

func NewReplaceCreateStep(iter *PlanIterator, old *resource.State,
	new *resource.Object, inputs resource.PropertyMap, reasons []resource.PropertyKey) *Step {
	contract.Assert(resource.HasURN(old))
	contract.Assert(!resource.HasURN(new))
	return &Step{iter: iter, op: OpReplaceCreate, urn: old.URN(), old: old, new: new, inputs: inputs, reasons: reasons}
}

func NewReplaceDeleteStep(iter *PlanIterator, old *resource.State) *Step {
	contract.Assert(resource.HasURN(old))
	return &Step{iter: iter, op: OpReplaceDelete, urn: old.URN(), old: old}
}

func (s *Step) Plan() *Plan                     { return s.iter.p }
func (s *Step) Iterator() *PlanIterator         { return s.iter }
func (s *Step) Op() StepOp                      { return s.op }
func (s *Step) URN() resource.URN               { return s.urn }
func (s *Step) Old() *resource.State            { return s.old }
func (s *Step) New() *resource.Object           { return s.new }
func (s *Step) Inputs() resource.PropertyMap    { return s.inputs }
func (s *Step) Outputs() resource.PropertyMap   { return s.outputs }
func (s *Step) Reasons() []resource.PropertyKey { return s.reasons }

func (s *Step) Provider() (plugin.Provider, error) {
	contract.Assert(s.old == nil || s.new == nil || s.old.Type() == s.new.Type())
	if s.old != nil {
		return s.Plan().Provider(s.old)
	}
	contract.Assert(s.new != nil)
	return s.Plan().Provider(s.new)
}

func (s *Step) Apply() (resource.Status, error) {
	// Fetch the provider.
	prov, err := s.Provider()
	if err != nil {
		return resource.StatusOK, err
	}

	// Now simply perform the operation of the right kind.
	switch s.op {
	case OpSame:
		// Just propagate the ID and output state to the live object and append to the snapshot.
		contract.Assert(s.old != nil)
		contract.Assert(s.new != nil)
		s.new.Update(s.urn, s.old.ID(), s.old.Outputs())
		s.iter.MarkStateSnapshot(s.old)
		s.iter.AppendStateSnapshot(s.old)

	case OpCreate, OpReplaceCreate:
		// Invoke the Create RPC function for this provider:
		contract.Assert(s.old == nil || s.op == OpReplaceCreate)
		contract.Assert(s.new != nil)
		t := s.new.Type()
		id, rst, err := prov.Create(t, s.inputs)
		if err != nil {
			return rst, err
		}
		contract.Assert(id != "")

		// Read the resource state back (to fetch outputs) and store everything on the live object.
		outs, err := prov.Get(t, id)
		if err != nil {
			return resource.StatusUnknown, err
		}
		s.outputs = outs
		state := s.new.Update(s.urn, id, outs)
		if s.old != nil {
			s.iter.MarkStateSnapshot(s.old)
		}
		s.iter.AppendStateSnapshot(state)

	case OpDelete, OpReplaceDelete:
		// Invoke the Delete RPC function for this provider:
		contract.Assert(s.old != nil)
		contract.Assert(s.new == nil)
		if rst, err := prov.Delete(s.old.Type(), s.old.ID()); err != nil {
			return rst, err
		}
		s.iter.MarkStateSnapshot(s.old)

	case OpUpdate:
		// Invoke the Update RPC function for this provider:
		contract.Assert(s.old != nil)
		contract.Assert(s.new != nil)
		t := s.old.Type()
		contract.Assert(t == s.new.Type())
		id := s.old.ID()
		contract.Assert(id != "")
		if rst, err := prov.Update(t, id, s.old.Inputs(), s.inputs); err != nil {
			return rst, err
		}

		// Now read the resource state back in case the update triggered cascading updates to other properties.
		outs, err := prov.Get(t, id)
		if err != nil {
			return resource.StatusUnknown, err
		}
		s.outputs = outs
		state := s.new.Update(s.urn, id, outs)
		s.iter.MarkStateSnapshot(s.old)
		s.iter.AppendStateSnapshot(state)

	default:
		contract.Failf("Unexpected step operation: %v", s.op)
	}

	return resource.StatusOK, nil
}

// Skip skips a step.  This is required even when just viewing a plan to ensure in-memory object states are correct.
// This factors in the correct differences in behavior depending on the kind of action being taken.
func (s *Step) Skip() {
	switch s.op {
	case OpSame:
		// In the case of a same, both ID and outputs are identical.
		s.new.Update(s.urn, s.old.ID(), s.old.Outputs())
	case OpCreate:
		// In the case of a create, we cannot possibly know the ID or output properties.  But we do know the URN.
		s.new.SetURN(s.urn)
	case OpUpdate:
		// In the case of an update, the ID is the same, however, the outputs remain unknown.
		s.new.SetURN(s.urn)
		s.new.SetID(s.old.ID())
	case OpReplaceCreate:
		// In the case of a replacement, we neither propagate the ID nor output properties.  This may be surprising,
		// however, it must be done this way since the entire resource will be deleted and recreated.  As a result, we
		// actually want the ID to be seen as having been updated (triggering cascading updates as appropriate).
	case OpDelete, OpReplaceDelete:
		// In the case of a deletion, there is no state to propagate: the new object doesn't even exist.
	default:
		contract.Failf("Unexpected step operation: %v", s.op)
	}
}

// StepOp represents the kind of operation performed by this step.
type StepOp string

const (
	OpSame          StepOp = "same"           // nothing to do.
	OpCreate        StepOp = "create"         // creating a new resource.
	OpUpdate        StepOp = "update"         // updating an existing resource.
	OpDelete        StepOp = "delete"         // deleting an existing resource.
	OpReplaceCreate StepOp = "replace"        // replacing a resource with a new one.
	OpReplaceDelete StepOp = "replace-delete" // the fine-grained replacement step to delete the old resource.
)

// StepOps contains the full set of step operation types.
var StepOps = []StepOp{
	OpSame,
	OpCreate,
	OpUpdate,
	OpDelete,
	OpReplaceCreate,
	OpReplaceDelete,
}

// Color returns a suggested color for lines of this op type.
func (op StepOp) Color() string {
	switch op {
	case OpSame:
		return ""
	case OpCreate:
		return colors.SpecAdded
	case OpDelete:
		return colors.SpecDeleted
	case OpUpdate:
		return colors.SpecChanged
	case OpReplaceCreate:
		return colors.SpecReplaced
	case OpReplaceDelete:
		return colors.SpecDeleted
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
		return op.Color() + "  "
	case OpReplaceCreate:
		return op.Color() + "~+"
	case OpReplaceDelete:
		return op.Color() + "~-"
	default:
		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
}

// Suffix returns a suggested suffix for lines of this op type.
func (op StepOp) Suffix() string {
	if op == OpUpdate || op == OpReplaceCreate {
		return colors.Reset // updates and replacements colorize individual lines
	}
	return ""
}
