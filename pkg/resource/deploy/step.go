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
	old     *resource.State        // the state of the resource before this step.
	new     *resource.Object       // the state of the resource after this step.
	inputs  resource.PropertyMap   // the input properties to use during the operation.
	outputs resource.PropertyMap   // the output properties calculated after the operation.
	reasons []resource.PropertyKey // the reasons for replacement, if applicable.
}

func NewCreateStep(iter *PlanIterator, new *resource.Object, inputs resource.PropertyMap) *Step {
	return &Step{iter: iter, op: OpCreate, new: new, inputs: inputs}
}

func NewDeleteStep(iter *PlanIterator, old *resource.State) *Step {
	return &Step{iter: iter, op: OpDelete, old: old}
}

func NewUpdateStep(iter *PlanIterator, old *resource.State,
	new *resource.Object, inputs resource.PropertyMap) *Step {
	return &Step{iter: iter, op: OpUpdate, old: old, new: new, inputs: inputs}
}

func NewReplaceCreateStep(iter *PlanIterator, old *resource.State,
	new *resource.Object, inputs resource.PropertyMap, reasons []resource.PropertyKey) *Step {
	return &Step{iter: iter, op: OpReplaceCreate, old: old, new: new, inputs: inputs, reasons: reasons}
}

func NewReplaceDeleteStep(iter *PlanIterator, old *resource.State) *Step {
	return &Step{iter: iter, op: OpReplaceDelete, old: old}
}

func (s *Step) Plan() *Plan                     { return s.iter.p }
func (s *Step) Iterator() *PlanIterator         { return s.iter }
func (s *Step) Op() StepOp                      { return s.op }
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
		state := s.new.Update(id, outs)
		s.iter.AppendStateSnapshot(state)

	case OpDelete, OpReplaceDelete:
		// Invoke the Delete RPC function for this provider:
		contract.Assert(s.old != nil)
		contract.Assert(s.new == nil)
		if rst, err := prov.Delete(s.old.Type(), s.old.ID()); err != nil {
			return rst, err
		}
		s.iter.RemoveStateSnapshot(s.old)

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
		state := s.new.Update(id, outs)
		s.iter.AppendStateSnapshot(state)

	default:
		contract.Failf("Unexpected step operation: %v", s.op)
	}

	return resource.StatusOK, nil
}

// StepOp represents the kind of operation performed by this step.
type StepOp string

const (
	OpCreate        StepOp = "create"         // creating a new resource.
	OpUpdate        StepOp = "update"         // updating an existing resource.
	OpDelete        StepOp = "delete"         // deleting an existing resource.
	OpReplaceCreate StepOp = "replace-create" // replacing a resource with a new one.
	OpReplaceDelete StepOp = "replace-delete" // the fine-grained replacement step to delete the old resource.
)

// StepOps contains the full set of step operation types.
var StepOps = []StepOp{
	OpCreate,
	OpUpdate,
	OpDelete,
	OpReplaceCreate,
	OpReplaceDelete,
}

// Color returns a suggested color for lines of this op type.
func (op StepOp) Color() string {
	switch op {
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
