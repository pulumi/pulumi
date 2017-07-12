// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag/colors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/plugin"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Step is a specification for a deployment operation.
type Step interface {
	Op() StepOp                      // the operation performed by this step.
	Plan() *Plan                     // the owning plan.
	Iterator() *PlanIterator         // the current plan iterator.
	Type() tokens.Type               // the type affected by this step.
	Pre() error                      // run any pre-execution steps.
	Apply() (resource.Status, error) // applies the action that this step represents.
	Skip() error                     // skips past this step (required when iterating a plan).
}

// ReadStep is a step that doesn't actually modify the target environment.  It only reads/queries from it.
type ReadStep interface {
	Step
	Resources() []*resource.Object // all resource objects returned by this step.
}

// MutatingStep is a step that, when performed, will actually modify/mutate the target environment and its resources.
type MutatingStep interface {
	Step
	URN() resource.URN             // the resource URN (for before and after).
	New() *resource.Object         // the state of the resource before performing this step.
	Old() *resource.State          // the state of the resource after performing this step.
	Inputs() resource.PropertyMap  // the input properties to use during the operation.
	Outputs() resource.PropertyMap // the output properties calculated during the operation.
}

// SameStep is a mutating step that does nothing.
type SameStep struct {
	iter   *PlanIterator        // the current plan iteration.
	old    *resource.State      // the state of the resource before this step.
	new    *resource.Object     // the state of the resource after this step.
	inputs resource.PropertyMap // the computed inputs supplied at creation time.
}

var _ MutatingStep = (*SameStep)(nil)

func NewSameStep(iter *PlanIterator, old *resource.State, new *resource.Object, inputs resource.PropertyMap) Step {
	contract.Assert(resource.HasURN(old))
	contract.Assert(!resource.HasURN(new))
	return &SameStep{
		iter:   iter,
		old:    old,
		new:    new,
		inputs: inputs,
	}
}

func (s *SameStep) Op() StepOp                    { return OpSame }
func (s *SameStep) Plan() *Plan                   { return s.iter.p }
func (s *SameStep) Iterator() *PlanIterator       { return s.iter }
func (s *SameStep) Type() tokens.Type             { return s.old.Type() }
func (s *SameStep) URN() resource.URN             { return s.old.URN() }
func (s *SameStep) Old() *resource.State          { return s.old }
func (s *SameStep) New() *resource.Object         { return s.new }
func (s *SameStep) Inputs() resource.PropertyMap  { return s.inputs }
func (s *SameStep) Outputs() resource.PropertyMap { return s.old.Outputs() }

func (s *SameStep) Pre() error {
	contract.Assert(s.old != nil)
	contract.Assert(s.new != nil)
	return nil
}

func (s *SameStep) Apply() (resource.Status, error) {
	// Just propagate the ID and output state to the live object and append to the snapshot.
	s.new.Update(s.old.URN(), s.old.ID(), s.old.Outputs())
	s.iter.MarkStateSnapshot(s.old)
	s.iter.AppendStateSnapshot(s.old)
	return resource.StatusOK, nil
}

func (s *SameStep) Skip() error {
	// In the case of a same, both ID and outputs are identical.
	s.new.Update(s.old.URN(), s.old.ID(), s.old.Outputs())
	return nil
}

// CreateStep is a mutating step that creates an entirely new resource.
type CreateStep struct {
	iter    *PlanIterator        // the current plan iteration.
	urn     resource.URN         // the resource URN being created.
	new     *resource.Object     // the state of the resource after this step.
	inputs  resource.PropertyMap // the input properties for the creation.
	outputs resource.PropertyMap // the output properties after creation.
}

var _ MutatingStep = (*CreateStep)(nil)

func NewCreateStep(iter *PlanIterator, urn resource.URN, new *resource.Object, inputs resource.PropertyMap) Step {
	contract.Assert(!resource.HasURN(new))
	return &CreateStep{
		iter:   iter,
		urn:    urn,
		new:    new,
		inputs: inputs,
	}
}

func (s *CreateStep) Op() StepOp                    { return OpCreate }
func (s *CreateStep) Plan() *Plan                   { return s.iter.p }
func (s *CreateStep) Iterator() *PlanIterator       { return s.iter }
func (s *CreateStep) Type() tokens.Type             { return s.new.Type() }
func (s *CreateStep) URN() resource.URN             { return s.urn }
func (s *CreateStep) Old() *resource.State          { return nil }
func (s *CreateStep) New() *resource.Object         { return s.new }
func (s *CreateStep) Inputs() resource.PropertyMap  { return s.inputs }
func (s *CreateStep) Outputs() resource.PropertyMap { return s.outputs }

func (s *CreateStep) Pre() error {
	contract.Assert(s.new != nil)
	return nil
}

func (s *CreateStep) Apply() (resource.Status, error) {
	t := s.new.Type()

	// Invoke the Create RPC function for this provider:
	prov, err := getProvider(s)
	if err != nil {
		return resource.StatusOK, err
	}
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
	s.iter.AppendStateSnapshot(state)
	return resource.StatusOK, nil
}

func (s *CreateStep) Skip() error {
	// In the case of a create, we cannot possibly know the ID or output properties.  But we do know the URN.
	s.new.SetURN(s.urn)
	return nil
}

// DeleteStep is a mutating step that deletes an existing resource.
type DeleteStep struct {
	iter     *PlanIterator   // the current plan iteration.
	old      *resource.State // the state of the existing resource.
	replaced bool            // true if part of a replacement.
}

var _ MutatingStep = (*DeleteStep)(nil)

func NewDeleteStep(iter *PlanIterator, old *resource.State, replaced bool) Step {
	contract.Assert(resource.HasURN(old))
	return &DeleteStep{
		iter:     iter,
		old:      old,
		replaced: replaced,
	}
}

func (s *DeleteStep) Op() StepOp                    { return OpDelete }
func (s *DeleteStep) Plan() *Plan                   { return s.iter.p }
func (s *DeleteStep) Iterator() *PlanIterator       { return s.iter }
func (s *DeleteStep) Type() tokens.Type             { return s.old.Type() }
func (s *DeleteStep) URN() resource.URN             { return s.old.URN() }
func (s *DeleteStep) Old() *resource.State          { return s.old }
func (s *DeleteStep) New() *resource.Object         { return nil }
func (s *DeleteStep) Inputs() resource.PropertyMap  { return s.old.Inputs() }
func (s *DeleteStep) Outputs() resource.PropertyMap { return s.old.Outputs() }
func (s *DeleteStep) Replaced() bool                { return s.replaced }

func (s *DeleteStep) Pre() error {
	contract.Assert(s.old != nil)
	return nil
}

func (s *DeleteStep) Apply() (resource.Status, error) {
	// Invoke the Delete RPC function for this provider:
	prov, err := getProvider(s)
	if err != nil {
		return resource.StatusOK, err
	}
	if rst, err := prov.Delete(s.old.Type(), s.old.ID()); err != nil {
		return rst, err
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
	iter    *PlanIterator        // the current plan iteration.
	old     *resource.State      // the state of the existing resource.
	new     *resource.Object     // the live resource object.
	inputs  resource.PropertyMap // the input properties for the update.
	outputs resource.PropertyMap // the output properties populated after updating.
}

var _ MutatingStep = (*UpdateStep)(nil)

func NewUpdateStep(iter *PlanIterator, old *resource.State,
	new *resource.Object, inputs resource.PropertyMap) Step {
	contract.Assert(resource.HasURN(old))
	contract.Assert(!resource.HasURN(new))
	return &UpdateStep{
		iter:   iter,
		old:    old,
		new:    new,
		inputs: inputs,
	}
}

func (s *UpdateStep) Op() StepOp                    { return OpUpdate }
func (s *UpdateStep) Plan() *Plan                   { return s.iter.p }
func (s *UpdateStep) Iterator() *PlanIterator       { return s.iter }
func (s *UpdateStep) Type() tokens.Type             { return s.old.Type() }
func (s *UpdateStep) URN() resource.URN             { return s.old.URN() }
func (s *UpdateStep) Old() *resource.State          { return s.old }
func (s *UpdateStep) New() *resource.Object         { return s.new }
func (s *UpdateStep) Inputs() resource.PropertyMap  { return s.inputs }
func (s *UpdateStep) Outputs() resource.PropertyMap { return s.outputs }

func (s *UpdateStep) Pre() error {
	contract.Assert(s.old != nil)
	contract.Assert(s.new != nil)
	contract.Assert(s.old.Type() == s.new.Type())
	contract.Assert(s.old.ID() != "")
	return nil
}

func (s *UpdateStep) Apply() (resource.Status, error) {
	t := s.old.Type()
	id := s.old.ID()

	// Invoke the Update RPC function for this provider:
	prov, err := getProvider(s)
	if err != nil {
		return resource.StatusOK, err
	}
	if rst, upderr := prov.Update(t, id, s.old.Inputs(), s.inputs); upderr != nil {
		return rst, upderr
	}

	// Now read the resource state back in case the update triggered cascading updates to other properties.
	outs, geterr := prov.Get(t, id)
	if geterr != nil {
		return resource.StatusUnknown, geterr
	}
	s.outputs = outs
	state := s.new.Update(s.old.URN(), id, outs)
	s.iter.MarkStateSnapshot(s.old)
	s.iter.AppendStateSnapshot(state)
	return resource.StatusOK, nil
}

func (s *UpdateStep) Skip() error {
	// In the case of an update, the ID is the same, however, the outputs remain unknown.
	s.new.SetURN(s.old.URN())
	s.new.SetID(s.old.ID())
	return nil
}

// ReplaceStep is a mutating step that updates an existing resource's state.
type ReplaceStep struct {
	iter    *PlanIterator          // the current plan iteration.
	old     *resource.State        // the state of the existing resource.
	new     *resource.Object       // the live resource object.
	inputs  resource.PropertyMap   // the input properties for the replacement.
	outputs resource.PropertyMap   // the output properties populated after replacing.
	reasons []resource.PropertyKey // the reasons for the replacement.
}

func NewReplaceStep(iter *PlanIterator, old *resource.State,
	new *resource.Object, inputs resource.PropertyMap, reasons []resource.PropertyKey) Step {
	contract.Assert(resource.HasURN(old))
	contract.Assert(!resource.HasURN(new))
	return &ReplaceStep{
		iter:    iter,
		old:     old,
		new:     new,
		inputs:  inputs,
		reasons: reasons,
	}
}

func (s *ReplaceStep) Op() StepOp                      { return OpReplace }
func (s *ReplaceStep) Plan() *Plan                     { return s.iter.p }
func (s *ReplaceStep) Iterator() *PlanIterator         { return s.iter }
func (s *ReplaceStep) Type() tokens.Type               { return s.old.Type() }
func (s *ReplaceStep) URN() resource.URN               { return s.old.URN() }
func (s *ReplaceStep) Old() *resource.State            { return s.old }
func (s *ReplaceStep) New() *resource.Object           { return s.new }
func (s *ReplaceStep) Inputs() resource.PropertyMap    { return s.inputs }
func (s *ReplaceStep) Outputs() resource.PropertyMap   { return s.outputs }
func (s *ReplaceStep) Reasons() []resource.PropertyKey { return s.reasons }

func (s *ReplaceStep) Pre() error {
	contract.Assert(s.old != nil)
	contract.Assert(s.new != nil)
	return nil
}

func (s *ReplaceStep) Apply() (resource.Status, error) {
	t := s.new.Type()

	// Invoke the Create RPC function for this provider:
	prov, err := getProvider(s)
	if err != nil {
		return resource.StatusOK, err
	}
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
	state := s.new.Update(s.old.URN(), id, outs)
	s.iter.MarkStateSnapshot(s.old)
	s.iter.AppendStateSnapshot(state)
	return resource.StatusOK, nil
}

func (s *ReplaceStep) Skip() error {
	// In the case of a replacement, we neither propagate the ID nor output properties.  This may be surprising,
	// however, it must be done this way since the entire resource will be deleted and recreated.  As a result, we
	// actually want the ID to be seen as having been updated (triggering cascading updates as appropriate).
	s.new.SetURN(s.old.URN())
	return nil
}

// QueryStep is a read-only step that queries for all resources of a specific type.
type QueryStep struct {
	iter    *PlanIterator
	t       symbols.Type
	objs    []*resource.Object
	outputs []resource.PropertyMap
}

var _ ReadStep = (*QueryStep)(nil)

func NewQueryStep(iter *PlanIterator, t symbols.Type, objs []*resource.Object) Step {
	return &QueryStep{
		iter: iter,
		t:    t,
		objs: objs,
	}
}

func (s *QueryStep) Op() StepOp                    { return OpGet }
func (s *QueryStep) Plan() *Plan                   { return s.iter.p }
func (s *QueryStep) Iterator() *PlanIterator       { return s.iter }
func (s *QueryStep) Type() tokens.Type             { return s.t.TypeToken() }
func (s *QueryStep) Resources() []*resource.Object { return s.objs }

func (s *QueryStep) Pre() error {
	prov, err := getProvider(s)
	if err != nil {
		return err
	}
	outs, err := prov.Query(s.Type())
	if err != nil {
		return err
	}

	s.outputs = outs
	for _, obj := range s.objs {
		if obj == nil {
			obj = resource.NewEmptyObject(s.t)
		}
	}
	for _, obj := range s.objs {
		for _, out := range outs {
			obj.SetProperties(out)
		}
	}
	for _, obj := range s.objs {
		s.iter.Produce(obj)
	}

	return nil
}

func (s *QueryStep) Apply() (resource.Status, error) {
	return resource.StatusOK, nil
}

func (s *QueryStep) Skip() error {
	return nil
}

// GetStep is a read-only step that queries for a single resource.
type GetStep struct {
	iter    *PlanIterator        // the current plan iteration.
	t       symbols.Type         // the type of resource to query.
	id      resource.ID          // the ID of the resource being sought.
	obj     *resource.Object     // the resource object read back from this operation.
	outputs resource.PropertyMap // the output properties populated after updating.
}

var _ ReadStep = (*GetStep)(nil)

func NewGetStep(iter *PlanIterator, t symbols.Type, id resource.ID, obj *resource.Object) Step {
	return &GetStep{
		iter: iter,
		t:    t,
		id:   id,
		obj:  obj,
	}
}

func (s *GetStep) Op() StepOp                    { return OpGet }
func (s *GetStep) Plan() *Plan                   { return s.iter.p }
func (s *GetStep) Iterator() *PlanIterator       { return s.iter }
func (s *GetStep) Type() tokens.Type             { return s.t.TypeToken() }
func (s *GetStep) Resources() []*resource.Object { return []*resource.Object{s.obj} }

func (s *GetStep) Pre() error {
	// Simply call through to the provider's Get API.
	id := s.id
	prov, err := getProvider(s)
	if err != nil {
		return err
	}
	outs, err := prov.Get(s.Type(), id)
	if err != nil {
		return err
	}
	s.outputs = outs

	// If no pre-existing object was supplied, create a new one.
	if s.obj == nil {
		s.obj = resource.NewEmptyObject(s.t)
	}

	// Populate the object's ID, properties, and URN with the state we read back.
	// TODO: it's not clear yet how to correctly populate the URN, given that the allocation context is unknown.
	s.obj.SetID(id)
	s.obj.SetProperties(outs)

	// Finally, the iterate must communicate the result back to the interpreter, by way of an unwind.
	s.iter.Produce(s.obj)

	return nil
}

func (s *GetStep) Apply() (resource.Status, error) {
	return resource.StatusOK, nil
}

func (s *GetStep) Skip() error {
	return nil
}

// getProvider fetches the provider for the given step.
func getProvider(s Step) (plugin.Provider, error) {
	return s.Plan().ProviderT(s.Type())
}

// StepOp represents the kind of operation performed by a step.  It evaluates to its string label.
type StepOp string

const (
	OpSame    StepOp = "same"    // nothing to do.
	OpCreate  StepOp = "create"  // creating a new resource.
	OpUpdate  StepOp = "update"  // updating an existing resource.
	OpDelete  StepOp = "delete"  // deleting an existing resource.
	OpReplace StepOp = "replace" // replacing a resource with a new one.
	OpGet     StepOp = "get"     // fetching a resource by ID or URN.
	OpQuery   StepOp = "query"   // querying a resource list by type and filter.
)

// StepOps contains the full set of step operation types.
var StepOps = []StepOp{
	OpSame,
	OpCreate,
	OpUpdate,
	OpDelete,
	OpReplace,
	OpGet,
	OpQuery,
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
	case OpReplace:
		return colors.SpecReplaced
	case OpGet, OpQuery:
		return colors.SpecRead
	default:
		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
}

// Prefix returns a suggested prefix for lines of this op type.
func (op StepOp) Prefix() string {
	switch op {
	case OpSame, OpGet, OpQuery:
		return op.Color() + "  "
	case OpCreate:
		return op.Color() + "+ "
	case OpDelete:
		return op.Color() + "- "
	case OpUpdate:
		return op.Color() + "~ "
	case OpReplace:
		return op.Color() + "+-"
	default:
		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
}

// Suffix returns a suggested suffix for lines of this op type.
func (op StepOp) Suffix() string {
	if op == OpUpdate || op == OpReplace || op == OpGet {
		return colors.Reset // updates and replacements colorize individual lines; get has none
	}
	return ""
}
