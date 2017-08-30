// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"github.com/golang/glog"
	goerr "github.com/pkg/errors"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/errors"
	"github.com/pulumi/pulumi-fabric/pkg/resource"
	"github.com/pulumi/pulumi-fabric/pkg/resource/plugin"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// TODO[pulumi/pulumi-fabric#106]: parallelism.

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
	src, err := p.source.Iterate()
	if err != nil {
		return nil, err
	}

	// Create an iterator that can be used to perform the planning process.
	return &PlanIterator{
		p:        p,
		src:      src,
		urns:     make(map[resource.URN]bool),
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

	urns     map[resource.URN]bool // URNs discovered.
	creates  map[resource.URN]bool // URNs discovered to be created.
	updates  map[resource.URN]bool // URNs discovered to be updated.
	replaces map[resource.URN]bool // URNs discovered to be replaced.
	deletes  map[resource.URN]bool // URNs discovered to be deleted.
	sames    map[resource.URN]bool // URNs discovered to be the same.

	stepqueue []Step                   // a queue of steps to drain.
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

// Next advances the plan by a single step, and returns the next step to be performed.  In doing so, it will perform
// evaluation of the program as much as necessary to determine the next step.  If there is no further action to be
// taken, Next will return a nil step pointer.
func (iter *PlanIterator) Next() (Step, error) {
	for !iter.done {
		if len(iter.stepqueue) > 0 {
			step := iter.stepqueue[0]
			iter.stepqueue = iter.stepqueue[1:]
			return step, nil
		} else if !iter.srcdone {
			goal, err := iter.src.Next()
			if err != nil {
				return nil, err
			} else if goal != nil {
				steps, err := iter.nextResourceSteps(goal)
				if err != nil {
					return nil, err
				}
				contract.Assert(len(steps) > 0)
				if len(steps) > 1 {
					iter.stepqueue = steps[1:]
				}
				return steps[0], nil
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

// nextResourceSteps produces one or more steps required to achieve the desired resource goal state, or nil if there
// aren't any steps to perform (in other words, the actual known state is equivalent to the goal state).  It is
// possible to return multiple steps if the current resource state necessitates it (e.g., replacements).
func (iter *PlanIterator) nextResourceSteps(goal SourceGoal) ([]Step, error) {
	// Fetch the provider for this resource type.
	res := goal.Resource()
	prov, err := iter.Provider(res.Type)
	if err != nil {
		return nil, err
	}

	var invalid bool // will be set to true if this object fails validation.

	// Use the resource goal state name to produce a globally unique URN.
	urn := resource.NewURN(iter.p.Target().Name, iter.p.source.Pkg(), res.Type, tokens.QName(res.Name))
	if iter.urns[urn] {
		invalid = true
		// TODO[pulumi/pulumi-framework#19]: improve this error message!
		iter.p.Diag().Errorf(errors.ErrorDuplicateResourceURN, urn)
	}
	iter.urns[urn] = true

	// Produce a new state object that we'll build up as operations are performed.  It begins with empty outputs.
	// Ultimately, this is what will get serialized into the checkpoint file.
	new := resource.NewState(res.Type, urn, "", res.Properties, nil, nil)

	// If there is an old resource, apply its default properties before going any further.
	old, hasold := iter.p.Olds()[urn]
	if hasold {
		new.Defaults = new.Defaults.Merge(old.Defaults)
		if glog.V(9) {
			for k, v := range old.Defaults {
				glog.V(9).Infof("Applying old default %v=%v", k, v)
			}
		}
	}

	// Ensure the provider is okay with this resource and see if it has any new defaults to contribute.
	inputs := new.AllInputs()
	defaults, failures, err := prov.Check(new.Type, inputs)
	if err != nil {
		return nil, err
	} else if iter.issueCheckErrors(new, urn, failures) {
		invalid = true
	}

	// If there are any new defaults, apply them, and use the combined view for various operations below.
	if defaults != nil {
		new.Defaults = new.Defaults.Merge(defaults)
		inputs = new.AllInputs() // refresh our snapshot.
	}

	// Next, give each analyzer -- if any -- a chance to inspect the resource too.
	for _, a := range iter.p.analyzers {
		analyzer, err := iter.p.ctx.Host.Analyzer(a)
		if err != nil {
			return nil, err
		}
		failures, err := analyzer.Analyze(new.Type, inputs)
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
	if hasold {
		contract.Assert(old != nil && old.Type == new.Type)

		// The resource exists in both new and old; it could be an update.  This constitutes an update if the old
		// and new properties don't match exactly.  It is also possible we'll need to replace the resource if the
		// update impact assessment says so.  In this case, the resource's ID will change, which might have a
		// cascading impact on subsequent updates too, since those IDs must trigger recreations, etc.
		oldinputs := old.AllInputs()
		if !oldinputs.DeepEquals(inputs) {
			// The properties changed; we need to figure out whether to do an update or replacement.
			diff, err := prov.Diff(old.Type, old.ID, oldinputs, inputs)
			if err != nil {
				return nil, err
			}
			// This is either an update or a replacement; check for the latter first, and handle it specially.
			if diff.Replace() {
				iter.replaces[urn] = true
				// If we are going to perform a replacement, we need to recompute the default values.  The above logic
				// had assumed that we were going to carry them over from the old resource, which is no longer true.
				defaults, failures, err := prov.Check(new.Type, new.Inputs)
				if err != nil {
					return nil, err
				} else if iter.issueCheckErrors(new, urn, failures) {
					return nil, goerr.New("One or more resource validation errors occurred; refusing to proceed")
				}
				new.Defaults = defaults
				if glog.V(7) {
					glog.V(7).Infof("Planner decided to replace '%v' (oldprops=%v inputs=%v)",
						urn, oldinputs, new.AllInputs())
				}
				return []Step{
					NewCreateReplacementStep(iter, goal, old, new, diff.ReplaceKeys),
					NewReplaceStep(iter, old, new, diff.ReplaceKeys),
				}, nil
			}
			iter.updates[urn] = true
			if glog.V(7) {
				glog.V(7).Infof("Planner decided to update '%v' (oldprops=%v inputs=%v",
					urn, oldinputs, new.AllInputs())
			}
			return []Step{NewUpdateStep(iter, goal, old, new)}, nil
		}

		// No need to update anything, the properties didn't change.
		iter.sames[urn] = true
		if glog.V(7) {
			glog.V(7).Infof("Planner decided not to update '%v' (same) (inputs=%v)", urn, new.AllInputs())
		}
		return []Step{NewSameStep(iter, goal, old, new)}, nil
	}

	// Otherwise, the resource isn't in the old map, so it must be a resource creation.
	iter.creates[urn] = true
	glog.V(7).Infof("Planner decided to create '%v' (inputs=%v)", urn, new.AllInputs())
	return []Step{NewCreateStep(iter, goal, new)}, nil
}

// issueCheckErrors prints any check errors to the diagnostics sink.
func (iter *PlanIterator) issueCheckErrors(new *resource.State, urn resource.URN,
	failures []plugin.CheckFailure) bool {
	if len(failures) == 0 {
		return false
	}
	inputs := new.AllInputs()
	for _, failure := range failures {
		if failure.Property != "" {
			iter.p.Diag().Errorf(errors.ErrorResourcePropertyInvalidValue,
				new.Type, urn.Name(), failure.Property, inputs[failure.Property], failure.Reason)
		} else {
			iter.p.Diag().Errorf(errors.ErrorResourceInvalid, new.Type, urn.Name(), failure.Reason)
		}
	}
	return true
}

// nextDeleteStep produces a new step that deletes a resource if necessary.
func (iter *PlanIterator) nextDeleteStep() Step {
	if len(iter.delqueue) > 0 {
		del := iter.delqueue[0]
		iter.delqueue = iter.delqueue[1:]
		urn := del.URN
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
			urn := res.URN
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

	return NewSnapshot(iter.p.Target().Name, resources, iter.p.source.Info())
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

// Provider fetches the provider for a given resource type, possibly lazily allocating the plugins for it.  If a
// provider could not be found, or an error occurred while creating it, a non-nil error is returned.
func (iter *PlanIterator) Provider(t tokens.Type) (plugin.Provider, error) {
	pkg := t.Package()
	return iter.p.ctx.Host.Provider(pkg)
}
