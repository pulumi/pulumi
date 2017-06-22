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
	"github.com/golang/glog"
	goerr "github.com/pkg/errors"

	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/plugin"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// TODO[pulumi/lumi#106]: parallelism.

// Apply performs all steps in the plan, calling out to the progress reporting functions as desired.  It returns four
// things: the resulting Snapshot, no matter whether an error occurs or not; an error, if something went wrong; the step
// that failed, if the error is non-nil; and finally the state of the resource modified in the failing step.
func (p *Plan) Apply(prog Progress) (PlanSummary, Step, resource.Status, error) {
	// Fetch a plan iterator and keep walking it until we are done.
	iter, err := p.Iterate()
	if err != nil {
		return nil, nil, resource.StatusOK, err
	}

	n := 1
	step, err := iter.Next()
	if err != nil {
		_ = iter.Close() // ignore close errors; the Next error trumps
		return nil, nil, resource.StatusOK, err
	}

	for step != nil {
		// Do the pre-step.
		rst := resource.StatusOK
		err := step.Pre()

		// Perform pre-application progress reporting.
		if prog != nil {
			prog.Before(step)
		}

		if err == nil {
			rst, err = step.Apply()
		}

		// Perform post-application progress reporting.
		if prog != nil {
			prog.After(step, rst, err)
		}

		// If an error occurred, exit early.
		if err != nil {
			glog.V(7).Infof("Plan step #%v failed [%v]: %v", n, step.Op(), err)
			_ = iter.Close() // ignore close errors; the Apply error trumps
			return iter, step, rst, err
		}

		glog.V(7).Infof("Plan step #%v succeeded [%v]", n, step.Op())
		step, err = iter.Next()
		if err != nil {
			glog.V(7).Infof("Advancing to plan step #%v failed: %v", n+1, err)
			_ = iter.Close() // ignore close errors; the Apply error trumps
			return iter, step, resource.StatusOK, err
		}
		n++
	}

	// Finally, return a summary and the resulting plan information.
	return iter, nil, resource.StatusOK, iter.Close()
}

// Iterate initializes and returns an iterator that can be used to step through a plan's individual steps.
func (p *Plan) Iterate() (*PlanIterator, error) {
	// Ask the source for its iterator.
	src, err := p.new.Iterate()
	if err != nil {
		return nil, err
	}

	// Create an iterator that can be used to perform the planning process.
	return &PlanIterator{
		p:        p,
		src:      src,
		creates:  make(map[resource.URN]bool),
		updates:  make(map[resource.URN]bool),
		replaces: make(map[resource.URN]bool),
		deletes:  make(map[resource.URN]bool),
		sames:    make(map[resource.URN]bool),
		dones:    make(map[*resource.State]bool),
	}, nil
}

// PlanSummary is an interface for summarizing the progress of a plan.
type PlanSummary interface {
	Steps() int
	Creates() map[resource.URN]bool
	Updates() map[resource.URN]bool
	Replaces() map[resource.URN]bool
	Deletes() map[resource.URN]bool
	Sames() map[resource.URN]bool
	Resources() []*resource.State
	Snap() *Snapshot
}

// PlanIterator can be used to step through and/or execute a plan's proposed actions.
type PlanIterator struct {
	p   *Plan          // the plan to which this iterator belongs.
	src SourceIterator // the iterator that fetches source resources.

	creates  map[resource.URN]bool // URNs discovered to be created.
	updates  map[resource.URN]bool // URNs discovered to be updated.
	replaces map[resource.URN]bool // URNs discovered to be replaced.
	deletes  map[resource.URN]bool // URNs discovered to be deleted.
	sames    map[resource.URN]bool // URNs discovered to be the same.

	delqueue  []*resource.State        // a queue of deletes left to perform.
	resources []*resource.State        // the resulting ordered resource states.
	dones     map[*resource.State]bool // true for each old state we're done with.

	srcdone bool // true if the source interpreter has been run to completion.
	done    bool // true if the planning and associated iteration has finished.
}

func (iter *PlanIterator) Plan() *Plan { return iter.p }
func (iter *PlanIterator) Steps() int {
	return len(iter.creates) + len(iter.updates) + len(iter.replaces) + len(iter.deletes)
}
func (iter *PlanIterator) Creates() map[resource.URN]bool  { return iter.creates }
func (iter *PlanIterator) Updates() map[resource.URN]bool  { return iter.updates }
func (iter *PlanIterator) Replaces() map[resource.URN]bool { return iter.replaces }
func (iter *PlanIterator) Deletes() map[resource.URN]bool  { return iter.deletes }
func (iter *PlanIterator) Sames() map[resource.URN]bool    { return iter.sames }
func (iter *PlanIterator) Resources() []*resource.State    { return iter.resources }
func (iter *PlanIterator) Dones() map[*resource.State]bool { return iter.dones }
func (iter *PlanIterator) Done() bool                      { return iter.done }

// Close terminates the iteration of this plan.
func (iter *PlanIterator) Close() error {
	return iter.src.Close()
}

// Produce is used to indicate that a new resource state has been read from a live environment.
func (iter *PlanIterator) Produce(res *resource.Object) {
	iter.src.Produce(res)
}

// Next advances the plan by a single step, and returns the next step to be performed.  In doing so, it will perform
// evaluation of the program as much as necessary to determine the next step.  If there is no further action to be
// taken, Next will return a nil step pointer.
func (iter *PlanIterator) Next() (Step, error) {
	for !iter.done {
		if !iter.srcdone {
			res, q, err := iter.src.Next()
			if err != nil {
				return nil, err
			} else if res != nil {
				step, err := iter.nextResourceStep(res)
				if err != nil {
					return nil, err
				}
				contract.Assert(step != nil)
				return step, nil
			} else if q != nil {
				step, err := iter.nextQueryStep(q)
				if err != nil {
					return nil, err
				}
				contract.Assert(step != nil)
				return step, nil
			}

			// If all returns are nil, the source is done, note it, and don't go back for more.  Add any deletions to be
			// performed, and then keep going 'round the next iteration of the loop so we can wrap up the planning.
			iter.srcdone = true
			iter.delqueue = iter.calculateDeletes()
		} else {
			// The interpreter has finished, so we need to now drain any deletions that piled up.
			if step := iter.nextDeleteStep(); step != nil {
				return step, nil
			}

			// Otherwise, if the deletes have quiesced, there is nothing remaining in this plan; leave.
			iter.done = true
			break
		}
	}
	return nil, nil
}

// nextResourceStep produces a new step for a given resource or nil if there isn't one to perform.
func (iter *PlanIterator) nextResourceStep(res *SourceAllocation) (Step, error) {
	// Take a moment in time snapshot of the live object's properties.
	new := res.Obj
	t := new.Type()
	inputs := new.CopyProperties()

	// Fetch the provider for this resource type.
	prov, err := iter.Provider(new)
	if err != nil {
		return nil, err
	}

	// Fetch the resource's name from its provider, and use it to construct a URN.
	name, err := prov.Name(t, inputs)
	if err != nil {
		return nil, err
	}
	urn := resource.NewURN(iter.p.Target().Name, res.Ctx, t, name)

	// First ensure the provider is okay with this resource.
	var invalid bool
	failures, err := prov.Check(new.Type(), inputs)
	if err != nil {
		return nil, err
	}
	for _, failure := range failures {
		invalid = true
		var v resource.PropertyValue
		if failure.Property != "" {
			v = inputs[failure.Property]
		}
		iter.p.Diag().Errorf(errors.ErrorResourcePropertyInvalidValue, urn.Name(), failure.Property, v, failure.Reason)
	}

	// Next, give each analyzer -- if any -- a chance to inspect the resource too.
	for _, a := range iter.p.analyzers {
		analyzer, err := iter.p.ctx.Host.Analyzer(a)
		if err != nil {
			return nil, err
		}
		failures, err := analyzer.Analyze(t, inputs)
		if err != nil {
			return nil, err
		}
		for _, failure := range failures {
			invalid = true
			iter.p.Diag().Errorf(errors.ErrorAnalyzeResourceFailure, a, urn, failure.Property, failure.Reason)
		}
	}

	// If the resource isn't valid, don't proceed any further.
	if invalid {
		return nil, goerr.New("One or more resource validation errors occurred; refusing to proceed")
	}

	// Now decide what to do, step-wise:
	//
	//     * If the URN exists in the old snapshot, and it has been updated,
	//         - Check whether the update requires replacement.
	//         - If yes, create a new copy, and mark it as having been replaced.
	//         - If no, simply update the existing resource in place.
	//
	//     * If the URN does not exist in the old snapshot, create the resource anew.
	//
	if old, hasold := iter.p.Olds()[urn]; hasold {
		contract.Assert(old != nil && old.Type() == new.Type())

		// The resource exists in both new and old; it could be an update.  This constitutes an update if the old
		// and new properties don't match exactly.  It is also possible we'll need to replace the resource if the
		// update impact assessment says so.  In this case, the resource's ID will change, which might have a
		// cascading impact on subsequent updates too, since those IDs must trigger recreations, etc.
		oldprops := old.Inputs()
		if oldprops.DeepEquals(inputs) {
			// No need to update anything, the properties didn't change.
			iter.sames[urn] = true
			glog.V(7).Infof("Planner decided not to update '%v'", urn)
			return NewSameStep(iter, old, new, inputs), nil
		}

		// The properties changed; we need to figure out whether to do an update or replacement.
		replacements, _, err := prov.InspectChange(t, old.ID(), oldprops, inputs)
		if err != nil {
			return nil, err
		} else if len(replacements) > 0 {
			iter.replaces[urn] = true
			glog.V(7).Infof("Planner decided to replace '%v' (oldprops=%v inputs=%v)", urn, oldprops, inputs)
			return NewReplaceStep(iter, old, new, inputs, replacements), nil
		}

		iter.updates[urn] = true
		glog.V(7).Infof("Planner decided to update '%v' (oldprops=%v inputs=%v", urn, oldprops, inputs)
		return NewUpdateStep(iter, old, new, inputs), nil
	}

	// Otherwise, the resource isn't in the old map, so it must be a resource creation.
	iter.creates[urn] = true
	glog.V(7).Infof("Planner decided to create '%v'", urn)
	return NewCreateStep(iter, urn, new, inputs), nil
}

// nextQueryStep produces a new query step that looks up a resource in some manner.
func (iter *PlanIterator) nextQueryStep(q *SourceQuery) (Step, error) {
	if id := q.GetID; id != "" {
		return NewGetStep(iter, q.Type, id, nil), nil
	}
	contract.Assert(q.QueryFilter != nil)
	contract.Failf("TODO[pulumi/lumi#83]: querying not yet supported")
	return nil, nil
}

// nextDeleteStep produces a new step that deletes a resource if necessary.
func (iter *PlanIterator) nextDeleteStep() Step {
	if len(iter.delqueue) > 0 {
		del := iter.delqueue[0]
		iter.delqueue = iter.delqueue[1:]
		urn := del.URN()
		iter.deletes[urn] = true
		if iter.replaces[urn] {
			glog.V(7).Infof("Planner decided to delete '%v' due to replacement", urn)
		} else {
			glog.V(7).Infof("Planner decided to delete '%v'", urn)
		}
		return NewDeleteStep(iter, del, iter.replaces[urn])
	}
	return nil
}

// calculateDeletes creates a list of deletes to perform.  This will include any resources in the snapshot that were
// not encountered in the input, along with any resources that were replaced.
func (iter *PlanIterator) calculateDeletes() []*resource.State {
	// To compute the deletion list, we must walk the list of old resources *backwards*.  This is because the list is
	// stored in dependency order, and earlier elements are possibly leaf nodes for later elements.  We must not delete
	// dependencies prior to their dependent nodes.
	var dels []*resource.State
	if prev := iter.p.prev; prev != nil {
		for i := len(prev.Resources) - 1; i >= 0; i-- {
			res := prev.Resources[i]
			urn := res.URN()
			contract.Assert(!iter.creates[urn])
			if (!iter.sames[urn] && !iter.updates[urn]) || iter.replaces[urn] {
				dels = append(dels, res)
			}
		}
	}
	return dels
}

// Snap returns a fresh snapshot that takes into account everything that has happened up till this point.  Namely, if a
// failure happens partway through, the untouched snapshot elements will be retained, while any updates will be
// preserved.  If no failure happens, the snapshot naturally reflects the final state of all resources.
func (iter *PlanIterator) Snap() *Snapshot {
	var resources []*resource.State

	// If we didn't finish the execution, we must produce a partial snapshot of old plus new states.
	if !iter.done {
		if prev := iter.p.prev; prev != nil {
			for _, res := range prev.Resources {
				if !iter.dones[res] {
					resources = append(resources, res)
				}
			}
		}
	}

	// Always add the new resoures afterwards that got produced during the evaluation of the current plan.
	resources = append(resources, iter.resources...)

	return NewSnapshot(iter.p.Target().Name, resources, iter.p.new.Info())
}

// MarkStateSnapshot marks an old state snapshot as being processed.  This is done to recover from failures partway
// through the application of a deployment plan.  Any old state that has not yet been recovered needs to be kept.
func (iter *PlanIterator) MarkStateSnapshot(state *resource.State) {
	iter.dones[state] = true
}

// AppendStateSnapshot appends a resource's state to the current snapshot.
func (iter *PlanIterator) AppendStateSnapshot(state *resource.State) {
	iter.resources = append(iter.resources, state)
}

// Provider fetches the provider for a given resource, possibly lazily allocating the plugins for it.  If a provider
// could not be found, or an error occurred while creating it, a non-nil error is returned.
func (iter *PlanIterator) Provider(res resource.Resource) (plugin.Provider, error) {
	t := res.Type()
	pkg := t.Package()
	return iter.p.ctx.Host.Provider(pkg)
}
