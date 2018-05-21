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
	"reflect"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

// Options controls the planning and deployment process.
type Options struct {
	Events   Events // an optional events callback interface.
	Parallel int    // the degree of parallelism for resource operations (<=1 for serial).
}

// Events is an interface that can be used to hook interesting engine/planning events.
type Events interface {
	OnResourceStepPre(step Step) (interface{}, error)
	OnResourceStepPost(ctx interface{}, step Step, status resource.Status, err error) error
	OnResourceOutputs(step Step) error
}

// Start initializes and returns an iterator that can be used to step through a plan's individual steps.
func (p *Plan) Start(opts Options) (*PlanIterator, error) {
	// Ask the source for its iterator.
	src, err := p.source.Iterate(opts)
	if err != nil {
		return nil, err
	}

	// Create an iterator that can be used to perform the planning process.
	return &PlanIterator{
		p:           p,
		opts:        opts,
		src:         src,
		urns:        make(map[resource.URN]bool),
		creates:     make(map[resource.URN]bool),
		updates:     make(map[resource.URN]bool),
		replaces:    make(map[resource.URN]bool),
		deletes:     make(map[resource.URN]bool),
		sames:       make(map[resource.URN]bool),
		pendingNews: make(map[resource.URN]Step),
		dones:       make(map[*resource.State]bool),
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
}

// PlanIterator can be used to step through and/or execute a plan's proposed actions.
type PlanIterator struct {
	p    *Plan          // the plan to which this iterator belongs.
	opts Options        // the options this iterator was created with.
	src  SourceIterator // the iterator that fetches source resources.

	urns     map[resource.URN]bool // URNs discovered.
	creates  map[resource.URN]bool // URNs discovered to be created.
	updates  map[resource.URN]bool // URNs discovered to be updated.
	replaces map[resource.URN]bool // URNs discovered to be replaced.
	deletes  map[resource.URN]bool // URNs discovered to be deleted.
	sames    map[resource.URN]bool // URNs discovered to be the same.

	pendingNews map[resource.URN]Step // a map of logical steps currently active.

	stepqueue []Step                   // a queue of steps to drain.
	delqueue  []Step                   // a queue of deletes left to perform.
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

// Apply performs a plan's step and records its result in the iterator's state.
func (iter *PlanIterator) Apply(step Step, preview bool) (resource.Status, error) {
	urn := step.URN()

	// If there is a pre-event, raise it.
	var eventctx interface{}
	if e := iter.opts.Events; e != nil {
		var eventerr error
		eventctx, eventerr = e.OnResourceStepPre(step)
		if eventerr != nil {
			return resource.StatusOK, errors.Wrapf(eventerr, "pre-step event returned an error")
		}
	}

	// Apply the step.
	logging.V(9).Infof("Applying step %v on %v (preview %v)", step.Op(), urn, preview)
	status, err := step.Apply(preview)

	// If there is no error, proceed to save the state; otherwise, go straight to the exit codepath.
	if err == nil {
		// If we have a state object, and this is a create or update, remember it, as we may need to update it later.
		if step.Logical() && step.New() != nil {
			if prior, has := iter.pendingNews[urn]; has {
				return resource.StatusOK,
					errors.Errorf("resource '%s' registered twice (%s and %s)", urn, prior.Op(), step.Op())
			}

			iter.pendingNews[urn] = step
		}
	}

	// If there is a post-event, raise it, and in any case, return the results.
	if e := iter.opts.Events; e != nil {
		if eventerr := e.OnResourceStepPost(eventctx, step, status, err); eventerr != nil {
			return status, errors.Wrapf(eventerr, "post-step event returned an error")
		}
	}

	// At this point, if err is not nil, we've already issued an error message through our
	// diag subsystem and we need to bail.
	//
	// This error message is ultimately what's going to be presented to the user at the top
	// level, so the message here is intentionally vague; we should have already presented
	// a more specific error message.
	if err != nil {
		if preview {
			return status, errors.New("preview failed")
		}

		return status, errors.New("update failed")
	}

	return status, nil
}

// Close terminates the iteration of this plan.
func (iter *PlanIterator) Close() error {
	return iter.src.Close()
}

// Next advances the plan by a single step, and returns the next step to be performed.  In doing so, it will perform
// evaluation of the program as much as necessary to determine the next step.  If there is no further action to be
// taken, Next will return a nil step pointer.
func (iter *PlanIterator) Next() (Step, error) {
outer:
	for !iter.done {
		if len(iter.stepqueue) > 0 {
			step := iter.stepqueue[0]
			iter.stepqueue = iter.stepqueue[1:]
			return step, nil
		} else if !iter.srcdone {
			event, err := iter.src.Next()
			if err != nil {
				return nil, err
			} else if event != nil {
				// If we have an event, drive the behavior based on which kind it is.
				switch e := event.(type) {
				case RegisterResourceEvent:
					// If the intent is to register a resource, compute the plan steps necessary to do so.
					steps, steperr := iter.makeRegisterResourceSteps(e)
					if steperr != nil {
						return nil, steperr
					}
					contract.Assert(len(steps) > 0)
					if len(steps) > 1 {
						iter.stepqueue = steps[1:]
					}
					return steps[0], nil
				case RegisterResourceOutputsEvent:
					// If the intent is to complete a prior resource registration, do so.  We do this by just
					// processing the request from the existing state, and do not expose our callers to it.
					if err := iter.registerResourceOutputs(e); err != nil {
						return nil, err
					}
					continue outer
				default:
					contract.Failf("Unrecognized intent from source iterator: %v", reflect.TypeOf(event))
				}
			}

			// If all returns are nil, the source is done, note it, and don't go back for more.  Add any deletions to be
			// performed, and then keep going 'round the next iteration of the loop so we can wrap up the planning.
			iter.srcdone = true
			iter.delqueue = iter.computeDeletes()
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

// diff returns a DiffResult for the given resource.
func (iter *PlanIterator) diff(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs, newOutputs,
	newProps resource.PropertyMap, prov plugin.Provider, refresh, allowUnknowns bool) (plugin.DiffResult, error) {

	// Workaround #1251: unexpected replaces.
	//
	// The legacy/desired behavior here is that if the provider-calculated inputs for a resource did not change,
	// then the resource itself should not change. Unfortunately, we (correctly?) pass the entire current state
	// of the resource to Diff, which includes calculated/output properties that may differ from those present
	// in the input properties. This can cause unexpected diffs.
	//
	// For now, simply apply the legacy diffing behavior before deferring to the provider.
	var hasChanges bool
	if refresh {
		hasChanges = !oldOutputs.DeepEquals(newOutputs)
	} else {
		hasChanges = !oldInputs.DeepEquals(newInputs)
	}
	if !hasChanges {
		return plugin.DiffResult{Changes: plugin.DiffNone}, nil
	}

	// If there is no provider for this resource, simply return a "diffs exist" result.
	if prov == nil {
		return plugin.DiffResult{Changes: plugin.DiffSome}, nil
	}

	// Grab the diff from the provider. At this point we know that there were changes to the Pulumi inputs, so if the
	// provider returns an "unknown" diff result, pretend it returned "diffs exist".
	diff, err := prov.Diff(urn, id, oldOutputs, newProps, allowUnknowns)
	if err != nil {
		return plugin.DiffResult{}, err
	}
	if diff.Changes == plugin.DiffUnknown {
		diff.Changes = plugin.DiffSome
	}
	return diff, nil
}

// makeRegisterResourceSteps produces one or more steps required to achieve the desired resource goal state, or nil if
// there aren't any steps to perform (in other words, the actual known state is equivalent to the goal state).  It is
// possible to return multiple steps if the current resource state necessitates it (e.g., replacements).
func (iter *PlanIterator) makeRegisterResourceSteps(e RegisterResourceEvent) ([]Step, error) {
	var invalid bool // will be set to true if this object fails validation.

	// Use the resource goal state name to produce a globally unique URN.
	goal := e.Goal()
	parentType := tokens.Type("")
	if p := goal.Parent; p != "" && p.Type() != resource.RootStackType {
		// Skip empty parents and don't use the root stack type; otherwise, use the full qualified type.
		parentType = p.QualifiedType()
	}

	urn := resource.NewURN(iter.p.Target().Name, iter.p.source.Project(), parentType, goal.Type, goal.Name)
	if iter.urns[urn] {
		invalid = true
		// TODO[pulumi/pulumi-framework#19]: improve this error message!
		iter.p.Diag().Errorf(diag.GetDuplicateResourceURNError(urn), urn)
	}
	iter.urns[urn] = true

	// Check for an old resource so that we can figure out if this is a create, delete, etc., and/or to diff.
	old, hasOld := iter.p.Olds()[urn]
	var oldInputs resource.PropertyMap
	var oldOutputs resource.PropertyMap
	if hasOld {
		oldInputs = old.Inputs
		oldOutputs = old.Outputs
	}

	// Produce a new state object that we'll build up as operations are performed.  Ultimately, this is what will
	// get serialized into the checkpoint file.  Normally there are no outputs, unless this is a refresh.
	props, inputs, outputs, new := iter.getResourcePropertyStates(urn, goal)

	// Fetch the provider for this resource type, assuming it isn't just a logical one.
	var prov plugin.Provider
	var err error
	if goal.Custom {
		if prov, err = iter.Provider(goal.Type); err != nil {
			return nil, err
		}
	}

	// See if we're performing a refresh update, which takes slightly different code-paths.
	refresh := iter.p.IsRefresh()

	// We only allow unknown property values to be exposed to the provider if we are performing an update preview.
	allowUnknowns := iter.p.preview && !refresh

	// If this isn't a refresh, ensure the provider is okay with this resource and fetch the inputs to pass to
	// subsequent methods.  If these are not inputs, we are just going to blindly store the outputs, so skip this.
	if prov != nil && !refresh {
		var failures []plugin.CheckFailure
		inputs, failures, err = prov.Check(urn, oldInputs, inputs, allowUnknowns)
		if err != nil {
			return nil, err
		} else if iter.issueCheckErrors(new, urn, failures) {
			invalid = true
		}
		props = inputs
		new.Inputs = inputs
	}

	// Next, give each analyzer -- if any -- a chance to inspect the resource too.
	for _, a := range iter.p.analyzers {
		var analyzer plugin.Analyzer
		analyzer, err = iter.p.ctx.Host.Analyzer(a)
		if err != nil {
			return nil, err
		} else if analyzer == nil {
			return nil, errors.Errorf("analyzer '%v' could not be loaded from your $PATH", a)
		}
		var failures []plugin.AnalyzeFailure
		failures, err = analyzer.Analyze(new.Type, props)
		if err != nil {
			return nil, err
		}
		for _, failure := range failures {
			invalid = true
			iter.p.Diag().Errorf(
				diag.GetAnalyzeResourceFailureError(urn), a, urn, failure.Property, failure.Reason)
		}
	}

	// If the resource isn't valid, don't proceed any further.
	if invalid {
		return nil, errors.New("One or more resource validation errors occurred; refusing to proceed")
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
	// If the URN exists in the old snapshot but we've recorded that we've deleted it already,
	// we will do a create-replacement in order to re-create the deleted resource.
	if hasOld && !iter.deletes[urn] {
		contract.Assert(old != nil && old.Type == new.Type)

		// Determine whether the change resulted in a diff.
		diff, err := iter.diff(urn, old.ID, oldInputs, oldOutputs, inputs, outputs, props, prov, refresh,
			allowUnknowns)
		if err != nil {
			return nil, err
		}

		// Ensure that we received a sensible response.
		if diff.Changes != plugin.DiffNone && diff.Changes != plugin.DiffSome {
			return nil, errors.Errorf(
				"unrecognized diff state for %s: %d", urn, diff.Changes)
		}

		// If there were changes, check for a replacement vs. an in-place update.
		if diff.Changes == plugin.DiffSome {
			if diff.Replace() {
				iter.replaces[urn] = true

				// If we are going to perform a replacement, we need to recompute the default values.  The above logic
				// had assumed that we were going to carry them over from the old resource, which is no longer true.
				if prov != nil && !refresh {
					var failures []plugin.CheckFailure
					inputs, failures, err = prov.Check(urn, nil, goal.Properties, allowUnknowns)
					if err != nil {
						return nil, err
					} else if iter.issueCheckErrors(new, urn, failures) {
						return nil, errors.New("One or more resource validation errors occurred; refusing to proceed")
					}
					new.Inputs = inputs
				}

				if logging.V(7) {
					logging.V(7).Infof("Planner decided to replace '%v' (oldprops=%v inputs=%v)",
						urn, oldInputs, new.Inputs)
				}

				// We have two approaches to performing replacements:
				//
				//     * CreateBeforeDelete: the default mode first creates a new instance of the resource, then
				//       updates all dependent resources to point to the new one, and finally after all of that,
				//       deletes the old resource.  This ensures minimal downtime.
				//
				//     * DeleteBeforeCreate: this mode can be used for resources that cannot be tolerate having
				//       side-by-side old and new instances alive at once.  This first deletes the resource and
				//       then creates the new one.  This may result in downtime, so is less preferred.  Note that
				//       until pulumi/pulumi#624 is resolved, we cannot safely perform this operation on resources
				//       that have dependent resources (we try to delete the resource while they refer to it).
				//
				// The provider is responsible for requesting which of these two modes to use.

				if diff.DeleteBeforeReplace {
					logging.V(7).Infof("Planner decided to delete-before-replacement for resource '%v'", urn)
					contract.Assert(iter.p.depGraph != nil)

					// DeleteBeforeCreate implies that we must immediately delete the resource. For correctness,
					// we must also eagerly delete all resources that depend directly or indirectly on the resource
					// being replaced.
					//
					// To do this, we'll utilize the dependency information contained in the snapshot, which is
					// interpreted by the DependencyGraph type.
					var steps []Step
					for _, dependentResource := range iter.p.depGraph.DependingOn(old) {
						// If we already deleted this resource due to some other DBR, don't do it again.
						if iter.deletes[dependentResource.URN] {
							continue
						}

						logging.V(7).Infof("Planner decided to delete '%v' due to dependence on condemned resource '%v'",
							dependentResource.URN, urn)
						steps = append(steps, NewDeleteReplacementStep(iter.p, dependentResource, false))

						// Mark the condemned resource as deleted. We won't know until later in the plan whether
						// or not we're going to be replacing this resource.
						iter.deletes[dependentResource.URN] = true
					}

					return append(steps,
						NewDeleteReplacementStep(iter.p, old, false),
						NewReplaceStep(iter.p, old, new, diff.ReplaceKeys, false),
						NewCreateReplacementStep(iter.p, e, old, new, diff.ReplaceKeys, false),
					), nil
				}

				return []Step{
					NewCreateReplacementStep(iter.p, e, old, new, diff.ReplaceKeys, true),
					NewReplaceStep(iter.p, old, new, diff.ReplaceKeys, true),
					// note that the delete step is generated "later" on, after all creates/updates finish.
				}, nil
			}

			// If we fell through, it's an update.
			iter.updates[urn] = true
			if logging.V(7) {
				logging.V(7).Infof("Planner decided to update '%v' (oldprops=%v inputs=%v", urn, oldInputs, new.Inputs)
			}
			return []Step{NewUpdateStep(iter.p, e, old, new, diff.StableKeys)}, nil
		}

		// No need to update anything, the properties didn't change.
		iter.sames[urn] = true
		if logging.V(7) {
			logging.V(7).Infof("Planner decided not to update '%v' (same) (inputs=%v)", urn, new.Inputs)
		}
		return []Step{NewSameStep(iter.p, e, old, new)}, nil
	}

	if iter.deletes[urn] {
		// this resource existed, but we deleted it earlier. we'll re-create it here.
		logging.V(7).Infof("Planner decided to re-create replaced resource '%v' deleted due to dependent DBR", urn)
		contract.Assert(!refresh)
		diff, err := iter.diff(urn, old.ID, oldInputs, oldOutputs, inputs, outputs, props, prov, false, allowUnknowns)
		if err != nil {
			return nil, err
		}

		// Unmark this resource as deleted, we now know it's being replaced instead.
		delete(iter.deletes, urn)
		iter.replaces[urn] = true
		return []Step{
			NewReplaceStep(iter.p, old, new, diff.ReplaceKeys, false),
			NewCreateReplacementStep(iter.p, e, old, new, diff.ReplaceKeys, false),
		}, nil
	}

	// Otherwise, the resource isn't in the old map or it was deleted earlier, so it must be a resource creation.
	iter.creates[urn] = true
	logging.V(7).Infof("Planner decided to create '%v' (inputs=%v)", urn, new.Inputs)
	return []Step{NewCreateStep(iter.p, e, new)}, nil
}

// getResourcePropertyStates returns the properties, inputs, outputs, and new resource state, given a goal state.
func (iter *PlanIterator) getResourcePropertyStates(urn resource.URN, goal *resource.Goal) (resource.PropertyMap,
	resource.PropertyMap, resource.PropertyMap, *resource.State) {
	props := goal.Properties
	var inputs resource.PropertyMap
	var outputs resource.PropertyMap
	if iter.p.IsRefresh() {
		// In the case of a refresh, we will preserve the old inputs (since we won't have any new ones).  Note
		// that this can lead to a state in which inputs could not have possibly produced the outputs, but this
		// will need to be reconciled manually by the programmer updating the program accordingly.
		if old, ok := iter.p.Olds()[urn]; ok {
			inputs = old.Inputs
		}
		outputs = props
	} else {
		// In the case of non-refreshes, outputs remain empty (they will be computed), but inputs are present.
		inputs = props
	}
	return props, inputs, outputs,
		resource.NewState(goal.Type, urn, goal.Custom, false, "",
			inputs, outputs, goal.Parent, goal.Protect, goal.Dependencies)
}

// issueCheckErrors prints any check errors to the diagnostics sink.
func (iter *PlanIterator) issueCheckErrors(new *resource.State, urn resource.URN,
	failures []plugin.CheckFailure) bool {
	if len(failures) == 0 {
		return false
	}
	inputs := new.Inputs
	for _, failure := range failures {
		if failure.Property != "" {
			iter.p.Diag().Errorf(diag.GetResourcePropertyInvalidValueError(urn),
				new.Type, urn.Name(), failure.Property, inputs[failure.Property], failure.Reason)
		} else {
			iter.p.Diag().Errorf(
				diag.GetResourceInvalidError(urn), new.Type, urn.Name(), failure.Reason)
		}
	}
	return true
}

func (iter *PlanIterator) registerResourceOutputs(e RegisterResourceOutputsEvent) error {
	// Look up the final state in the pending registration list.
	urn := e.URN()
	reg, has := iter.pendingNews[urn]
	contract.Assertf(has, "cannot complete a resource '%v' whose registration isn't pending", urn)
	contract.Assertf(reg != nil, "expected a non-nil resource step ('%v')", urn)
	delete(iter.pendingNews, urn)

	// Unconditionally set the resource's outputs to what was provided.  This intentionally overwrites whatever
	// might already be there, since otherwise "deleting" outputs would have no affect.
	outs := e.Outputs()
	logging.V(7).Infof("Registered resource outputs %s: old=#%d, new=#%d", urn, len(reg.New().Outputs), len(outs))
	reg.New().Outputs = e.Outputs()

	// If there is an event subscription for finishing the resource, execute them.
	if e := iter.opts.Events; e != nil {
		if eventerr := e.OnResourceOutputs(reg); eventerr != nil {
			return errors.Wrapf(eventerr, "resource complete event returned an error")
		}
	}

	// Finally, let the language provider know that we're done processing the event.
	e.Done()
	return nil
}

// computeDeletes creates a list of deletes to perform.  This will include any resources in the snapshot that were
// not encountered in the input, along with any resources that were replaced.
func (iter *PlanIterator) computeDeletes() []Step {
	// To compute the deletion list, we must walk the list of old resources *backwards*.  This is because the list is
	// stored in dependency order, and earlier elements are possibly leaf nodes for later elements.  We must not delete
	// dependencies prior to their dependent nodes.
	var dels []Step
	if prev := iter.p.prev; prev != nil {
		for i := len(prev.Resources) - 1; i >= 0; i-- {
			// If this resource is explicitly marked for deletion or wasn't seen at all, delete it.
			res := prev.Resources[i]
			if res.Delete {
				logging.V(7).Infof("Planner decided to delete '%v' due to replacement", res.URN)
				contract.Assert(!iter.deletes[res.URN])
				iter.deletes[res.URN] = true
				dels = append(dels, NewDeleteReplacementStep(iter.p, res, true))
			} else if !iter.sames[res.URN] && !iter.updates[res.URN] && !iter.replaces[res.URN] {
				logging.V(7).Infof("Planner decided to delete '%v'", res.URN)
				iter.deletes[res.URN] = true
				dels = append(dels, NewDeleteStep(iter.p, res))
			}
		}
	}
	return dels
}

// nextDeleteStep produces a new step that deletes a resource if necessary.
func (iter *PlanIterator) nextDeleteStep() Step {
	if len(iter.delqueue) > 0 {
		del := iter.delqueue[0]
		iter.delqueue = iter.delqueue[1:]
		return del
	}
	return nil
}

// Provider fetches the provider for a given resource type, possibly lazily allocating the plugins for it.  If a
// provider could not be found, or an error occurred while creating it, a non-nil error is returned.
func (iter *PlanIterator) Provider(t tokens.Type) (plugin.Provider, error) {
	pkg := t.Package()
	prov, err := iter.p.Provider(pkg)
	if err != nil {
		return nil, err
	} else if prov == nil {
		return nil, errors.Errorf("could not load resource provider for package '%v' from $PATH", pkg)
	}
	return prov, nil
}
