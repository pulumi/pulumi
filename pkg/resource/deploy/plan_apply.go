// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"reflect"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/version"
	"github.com/pulumi/pulumi/pkg/workspace"
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
	Snap() *Snapshot
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
	glog.V(9).Infof("Applying step %v on %v (preview %v)", step.Op(), urn, preview)
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
					steps, steperr := iter.makeRegisterResouceSteps(e)
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

// makeRegisterResouceSteps produces one or more steps required to achieve the desired resource goal state, or nil if
// there aren't any steps to perform (in other words, the actual known state is equivalent to the goal state).  It is
// possible to return multiple steps if the current resource state necessitates it (e.g., replacements).
func (iter *PlanIterator) makeRegisterResouceSteps(e RegisterResourceEvent) ([]Step, error) {
	var invalid bool // will be set to true if this object fails validation.

	// Use the resource goal state name to produce a globally unique URN.
	res := e.Goal()
	parentType := tokens.Type("")
	if res.Parent != "" && res.Parent.Type() != resource.RootStackType {
		// Skip empty parents and don't use the root stack type; otherwise, use the full qualified type.
		parentType = res.Parent.QualifiedType()
	}

	urn := resource.NewURN(iter.p.Target().Name, iter.p.source.Project(), parentType, res.Type, res.Name)
	if iter.urns[urn] {
		invalid = true
		// TODO[pulumi/pulumi-framework#19]: improve this error message!
		iter.p.Diag().Errorf(diag.GetDuplicateResourceURNError(urn), urn)
	}
	iter.urns[urn] = true

	// Produce a new state object that we'll build up as operations are performed.  It begins with empty outputs.
	// Ultimately, this is what will get serialized into the checkpoint file.
	new := resource.NewState(res.Type, urn, res.Custom, false, "", res.Properties, nil,
		res.Parent, res.Protect, res.Dependencies)

	// Check for an old resource before going any further.
	old, hasOld := iter.p.Olds()[urn]
	var olds resource.PropertyMap
	var oldState resource.PropertyMap
	if hasOld {
		olds = old.Inputs
		oldState = old.All()
	}

	// Fetch the provider for this resource type, assuming it isn't just a logical one.
	var prov plugin.Provider
	var err error
	if res.Custom {
		if prov, err = iter.Provider(res.Type); err != nil {
			return nil, err
		}
	}

	// We only allow unknown property values to be exposed to the provider if we are performing a preview.
	allowUnknowns := iter.p.preview

	// Ensure the provider is okay with this resource and fetch the inputs to pass to subsequent methods.
	news, inputs := new.Inputs, new.Inputs
	if prov != nil {
		var failures []plugin.CheckFailure
		inputs, failures, err = prov.Check(urn, olds, news, allowUnknowns)
		if err != nil {
			return nil, err
		} else if iter.issueCheckErrors(new, urn, failures) {
			invalid = true
		}
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
		failures, err = analyzer.Analyze(new.Type, inputs)
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
	if hasOld {
		contract.Assert(old != nil && old.Type == new.Type)

		// The resource exists in both new and old; it could be an update.  This constitutes an update if the old
		// and new properties don't match exactly.  It is also possible we'll need to replace the resource if the
		// update impact assessment says so.  In this case, the resource's ID will change, which might have a
		// cascading impact on subsequent updates too, since those IDs must trigger recreations, etc.
		var diff plugin.DiffResult
		if prov != nil {
			if diff, err = prov.Diff(urn, old.ID, oldState, inputs, allowUnknowns); err != nil {
				return nil, err
			}
		}

		// Determine whether the change resulted in a diff.  Our legacy behavior here entailed actually performing
		// diffs of state on the Pulumi side, whereas our new behavior is to defer to the provider to decide.
		var hasChanges bool
		switch diff.Changes {
		case plugin.DiffSome:
			hasChanges = true
		case plugin.DiffNone:
			hasChanges = false
		case plugin.DiffUnknown:
			// This is legacy behavior; just use the DeepEquals function to diff on the Pulumi side.
			hasChanges = !olds.DeepEquals(inputs)
		default:
			return nil, errors.Errorf(
				"resource provider for %s replied with unrecognized diff state: %d", urn, diff.Changes)
		}

		// If this is an update, create the necessary step; otherwise, it's the same.
		if hasChanges {
			if diff.Replace() {
				iter.replaces[urn] = true

				// If we are going to perform a replacement, we need to recompute the default values.  The above logic
				// had assumed that we were going to carry them over from the old resource, which is no longer true.
				if prov != nil {
					var failures []plugin.CheckFailure
					inputs, failures, err = prov.Check(urn, nil, news, allowUnknowns)
					if err != nil {
						return nil, err
					} else if iter.issueCheckErrors(new, urn, failures) {
						return nil, errors.New("One or more resource validation errors occurred; refusing to proceed")
					}
					new.Inputs = inputs
				}

				if glog.V(7) {
					glog.V(7).Infof("Planner decided to replace '%v' (oldprops=%v inputs=%v)",
						urn, olds, new.Inputs)
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
					return []Step{
						NewDeleteReplacementStep(iter, old, false),
						NewReplaceStep(iter, old, new, diff.ReplaceKeys, false),
						NewCreateReplacementStep(iter, e, old, new, diff.ReplaceKeys, false),
					}, nil
				}

				return []Step{
					NewCreateReplacementStep(iter, e, old, new, diff.ReplaceKeys, true),
					NewReplaceStep(iter, old, new, diff.ReplaceKeys, true),
					// note that the delete step is generated "later" on, after all creates/updates finish.
				}, nil
			}

			// If we fell through, it's an update.
			iter.updates[urn] = true
			if glog.V(7) {
				glog.V(7).Infof("Planner decided to update '%v' (oldprops=%v inputs=%v", urn, olds, new.Inputs)
			}
			return []Step{NewUpdateStep(iter, e, old, new, diff.StableKeys)}, nil
		}

		// No need to update anything, the properties didn't change.
		iter.sames[urn] = true
		if glog.V(7) {
			glog.V(7).Infof("Planner decided not to update '%v' (same) (inputs=%v)", urn, new.Inputs)
		}
		return []Step{NewSameStep(iter, e, old, new)}, nil
	}

	// Otherwise, the resource isn't in the old map, so it must be a resource creation.
	iter.creates[urn] = true
	glog.V(7).Infof("Planner decided to create '%v' (inputs=%v)", urn, new.Inputs)
	return []Step{NewCreateStep(iter, e, new)}, nil
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
	glog.V(7).Infof("Registered resource outputs %s: old=#%d, new=#%d", urn, len(reg.New().Outputs), len(outs))
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
				glog.V(7).Infof("Planner decided to delete '%v' due to replacement", res.URN)
				iter.deletes[res.URN] = true
				dels = append(dels, NewDeleteReplacementStep(iter, res, true))
			} else if !iter.sames[res.URN] && !iter.updates[res.URN] && !iter.replaces[res.URN] {
				glog.V(7).Infof("Planner decided to delete '%v'", res.URN)
				iter.deletes[res.URN] = true
				dels = append(dels, NewDeleteStep(iter, res))
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

// Snap returns a fresh snapshot that takes into account everything that has happened up till this point.  Namely, if a
// failure happens partway through, the untouched snapshot elements will be retained, while any updates will be
// preserved.  If no failure happens, the snapshot naturally reflects the final state of all resources.
func (iter *PlanIterator) Snap() *Snapshot {
	// At this point we have two resource DAGs. One of these is the base DAG for this plan; the other is the current DAG
	// for this plan. Any resource r may be present in both DAGs. In order to produce a snapshot, we need to merge these
	// DAGs such that all resource dependencies are correctly preserved. Conceptually, the merge proceeds as follows:
	//
	// - Begin with an empty merged DAG.
	// - For each resource r in the current DAG, insert r and its outgoing edges into the merged DAG.
	// - For each resource r in the base DAG:
	//     - If r is in the merged DAG, we are done: if the resource is in the merged DAG, it must have been in the
	//       current DAG, which accurately captures its current dependencies.
	//     - If r is not in the merged DAG, insert it and its outgoing edges into the merged DAG.
	//
	// Physically, however, each DAG is represented as list of resources without explicit dependency edges. In place of
	// edges, it is assumed that the list represents a valid topological sort of its source DAG. Thus, any resource r at
	// index i in a list L must be assumed to be dependent on all resources in L with index j s.t. j < i. Due to this
	// representation, we implement the algorithm above as follows to produce a merged list that represents a valid
	// topological sort of the merged DAG:
	//
	// - Begin with an empty merged list.
	// - For each resource r in the current list, append r to the merged list. r must be in a correct location in the
	//   merged list, as its position relative to its assumed dependencies has not changed.
	// - For each resource r in the base list:
	//     - If r is in the merged list, we are done by the logic given in the original algorithm.
	//     - If r is not in the merged list, append r to the merged list. r must be in a correct location in the merged
	//       list:
	//         - If any of r's dependencies were in the current list, they must already be in the merged list and their
	//           relative order w.r.t. r has not changed.
	//         - If any of r's dependencies were not in the current list, they must already be in the merged list, as
	//           they would have been appended to the list before r.

	// Start with a copy of the resources produced during the evaluation of the current plan.
	resources := make([]*resource.State, len(iter.resources))
	copy(resources, iter.resources)

	// If the plan has not finished executing, append any resources from the base plan that were not produced by the
	// current plan.
	if !iter.done {
		if prev := iter.p.prev; prev != nil {
			for _, res := range prev.Resources {
				if !iter.dones[res] {
					resources = append(resources, res)
				}
			}
		}
	}

	// Now produce a manifest and snapshot.
	v, plugs := iter.SnapVersions()
	manifest := Manifest{
		Time:    time.Now(),
		Version: v,
		Plugins: plugs,
	}
	manifest.Magic = manifest.NewMagic()
	return NewSnapshot(iter.p.Target().Name, manifest, resources)
}

// SnapVersions returns all versions used in the generation of this snapshot.  Note that no attempt is made to
// "merge" with old version information.  So, if a checkpoint doesn't end up loading all of the possible plugins
// it could ever load -- e.g., due to a failure -- there will be some resources in the checkpoint snapshot that
// were loaded by plugins that never got loaded this time around.  In other words, this list is not stable.
func (iter *PlanIterator) SnapVersions() (string, []workspace.PluginInfo) {
	return version.Version, iter.p.ctx.Host.ListPlugins()
}

// MarkStateSnapshot marks an old state snapshot as being processed.  This is done to recover from failures partway
// through the application of a deployment plan.  Any old state that has not yet been recovered needs to be kept.
func (iter *PlanIterator) MarkStateSnapshot(state *resource.State) {
	contract.Assert(state != nil)
	iter.dones[state] = true
	glog.V(9).Infof("Marked old state snapshot as done: %v", state.URN)
}

// AppendStateSnapshot appends a resource's state to the current snapshot.
func (iter *PlanIterator) AppendStateSnapshot(state *resource.State) {
	contract.Assert(state != nil)
	iter.resources = append(iter.resources, state)
	glog.V(9).Infof("Appended new state snapshot to be written: %v", state.URN)
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
