// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"reflect"
	"sort"
	"time"

	"github.com/golang/glog"
	goerr "github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/compiler/errors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Options controls the planning and deployment process.
type Options struct {
	Parallel int // the degree of parallelism for resource operations (<=1 for serial).
}

// Start initializes and returns an iterator that can be used to step through a plan's individual steps.
func (p *Plan) Start(opts Options) (*PlanIterator, error) {
	// First, configure all providers based on the target configuration map.
	if err := p.configure(); err != nil {
		return nil, err
	}

	// Next, ask the source for its iterator.
	src, err := p.source.Iterate(opts)
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
		regs:     make(map[resource.URN]*FinalState),
		dones:    make(map[*resource.State]bool),
	}, nil
}

func (p *Plan) configure() error {
	var pkgs []string
	pkgconfigs := make(map[tokens.Package]map[tokens.ModuleMember]string)
	for k, v := range p.target.Config {
		pkg := k.Package()
		pkgs = append(pkgs, string(pkg))
		pkgconfig, has := pkgconfigs[pkg]
		if !has {
			pkgconfig = make(map[tokens.ModuleMember]string)
			pkgconfigs[pkg] = pkgconfig
		}
		pkgconfig[k] = v
	}
	sort.Strings(pkgs)
	initialized := make(map[string]bool)
	for _, pkg := range pkgs {
		if _, ready := initialized[pkg]; ready {
			continue
		}
		pkgt := tokens.Package(pkg)
		prov, err := p.Provider(pkgt)
		if err != nil {
			return goerr.Wrapf(err, "failed to get pkg '%v' resource provider", pkg)
		} else if prov != nil {
			// Note that it's legal for a provider to be missing for this package.  This simply indicates that
			// the configuration variable affects the program/package, and not a Go provider.
			if err = prov.Configure(pkgconfigs[pkgt]); err != nil {
				return goerr.Wrapf(err, "failed to configure pkg '%v' resource provider", pkg)
			}
		}
		initialized[pkg] = true
	}
	return nil
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

	stepqueue []Step                       // a queue of steps to drain.
	delqueue  []*resource.State            // a queue of deletes left to perform.
	resources []*resource.State            // the resulting ordered resource states.
	regs      map[resource.URN]*FinalState // a map of pending registrations not yet complete.
	dones     map[*resource.State]bool     // true for each old state we're done with.

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
func (iter *PlanIterator) Apply(step Step, skip bool) (resource.Status, error) {
	// Ensure we don't have a pending registration for this resource already.
	urn := step.URN()
	if _, has := iter.regs[urn]; has {
	}

	// Apply the step.
	status, state, err := step.Apply(skip)
	if err != nil {
		return status, err
	}

	// Now stash away its resulting state for safe-keeping; we will spill it to the checkpoint later on.
	if state != nil {
		iter.regs[urn] = state
	}
	return resource.StatusOK, nil
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
			intent, err := iter.src.Next()
			if err != nil {
				return nil, err
			} else if intent != nil {
				// If we have an intent, drive the behavior based on which kind it is.
				switch it := intent.(type) {
				case RegisterIntent:
					// If the intent is to register a resource, compute the plan steps necessary to do so.
					steps, steperr := iter.computeResourceSteps(it)
					if err != nil {
						return nil, steperr
					}
					contract.Assert(len(steps) > 0)
					if len(steps) > 1 {
						iter.stepqueue = steps[1:]
					}
					return steps[0], nil
				case CompleteIntent:
					// If the intent is to complete a prior resource registration, do so.  We do this by just
					// processing the request from the existing state, and do not expose our callers to it.
					if err = iter.completeResource(it); err != nil {
						return nil, err
					}
					continue outer
				default:
					contract.Failf("Unrecognized intent from source iterator: %v", reflect.TypeOf(intent))
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

// computeResourceSteps produces one or more steps required to achieve the desired resource goal state, or nil if there
// aren't any steps to perform (in other words, the actual known state is equivalent to the goal state).  It is
// possible to return multiple steps if the current resource state necessitates it (e.g., replacements).
func (iter *PlanIterator) computeResourceSteps(reg RegisterIntent) ([]Step, error) {
	var invalid bool // will be set to true if this object fails validation.

	// Use the resource goal state name to produce a globally unique URN.
	res := reg.Goal()
	urn := resource.NewURN(iter.p.Target().Name, iter.p.source.Pkg(), res.Type, res.Name)
	if iter.urns[urn] {
		invalid = true
		// TODO[pulumi/pulumi-framework#19]: improve this error message!
		iter.p.Diag().Errorf(errors.ErrorDuplicateResourceURN, urn)
	}
	iter.urns[urn] = true

	// Produce a new state object that we'll build up as operations are performed.  It begins with empty outputs.
	// Ultimately, this is what will get serialized into the checkpoint file.
	new := resource.NewState(res.Type, urn, res.Custom, false, "", res.Properties, nil, nil, res.Parent)

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

	// Fetch the provider for this resource type, assuming it isn't just a logical one.
	var prov plugin.Provider
	var err error
	if res.Custom {
		if prov, err = iter.Provider(res.Type); err != nil {
			return nil, err
		}
	}

	inputs := new.AllInputs()

	// Ensure the provider is okay with this resource and see if it has any new defaults to contribute.
	var defaults resource.PropertyMap
	if prov != nil {
		var failures []plugin.CheckFailure
		defaults, failures, err = prov.Check(urn, inputs)
		if err != nil {
			return nil, err
		} else if iter.issueCheckErrors(new, urn, failures) {
			invalid = true
		}
	}

	// If there are any new defaults, apply them, and use the combined view for various operations below.
	if defaults != nil {
		new.Defaults = new.Defaults.Merge(defaults)
		inputs = new.AllInputs() // refresh our snapshot.
	}

	// Next, give each analyzer -- if any -- a chance to inspect the resource too.
	for _, a := range iter.p.analyzers {
		var analyzer plugin.Analyzer
		analyzer, err = iter.p.ctx.Host.Analyzer(a)
		if err != nil {
			return nil, err
		} else if analyzer == nil {
			return nil, goerr.Errorf("analyzer '%v' could not be loaded from your $PATH", a)
		}
		var failures []plugin.AnalyzeFailure
		failures, err = analyzer.Analyze(new.Type, inputs)
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
			var diff plugin.DiffResult
			if prov != nil {
				if diff, err = prov.Diff(urn, old.ID, oldinputs, inputs); err != nil {
					return nil, err
				}
			}

			// This is either an update or a replacement; check for the latter first, and handle it specially.
			if diff.Replace() {
				iter.replaces[urn] = true

				// If we are going to perform a replacement, we need to recompute the default values.  The above logic
				// had assumed that we were going to carry them over from the old resource, which is no longer true.
				if prov != nil {
					var failures []plugin.CheckFailure
					defaults, failures, err = prov.Check(urn, new.Inputs)
					if err != nil {
						return nil, err
					} else if iter.issueCheckErrors(new, urn, failures) {
						return nil, goerr.New("One or more resource validation errors occurred; refusing to proceed")
					}
				}

				new.Defaults = defaults
				if glog.V(7) {
					glog.V(7).Infof("Planner decided to replace '%v' (oldprops=%v inputs=%v)",
						urn, oldinputs, new.AllInputs())
				}

				return []Step{
					NewCreateReplacementStep(iter, reg, old, new, diff.ReplaceKeys),
					NewReplaceStep(iter, old, new, diff.ReplaceKeys),
				}, nil
			}

			// If we fell through, it's an update.
			iter.updates[urn] = true
			if glog.V(7) {
				glog.V(7).Infof("Planner decided to update '%v' (oldprops=%v inputs=%v",
					urn, oldinputs, new.AllInputs())
			}
			return []Step{NewUpdateStep(iter, reg, old, new, diff.StableKeys)}, nil
		}

		// No need to update anything, the properties didn't change.
		iter.sames[urn] = true
		if glog.V(7) {
			glog.V(7).Infof("Planner decided not to update '%v' (same) (inputs=%v)", urn, new.AllInputs())
		}
		return []Step{NewSameStep(iter, reg, old, new)}, nil
	}

	// Otherwise, the resource isn't in the old map, so it must be a resource creation.
	iter.creates[urn] = true
	glog.V(7).Infof("Planner decided to create '%v' (inputs=%v)", urn, new.AllInputs())
	return []Step{NewCreateStep(iter, reg, new)}, nil
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

func (iter *PlanIterator) completeResource(c CompleteIntent) error {
	// Look up the final state in the pending registration list.
	urn := c.URN()
	final, has := iter.regs[urn]
	if !has {
		return goerr.Errorf("cannot complete a resource '%v' whose registration isn't pending", urn)
	}
	contract.Assertf(final != nil, "expected a non-nil final resource state ('%v')", urn)
	delete(iter.regs, urn)

	// Complete the step and get back our final state.

	// If there are any extra properties to add to the outputs, do it now.
	if extras := c.Extras(); extras != nil {
		final.State.AddExtras(extras)
	}

	// Add the final state snapshot for this resource so that it gets serialized appropriately.
	iter.CompleteStateSnapshot(urn, final.State)

	// Now communicate the results back to the language provider, who is waiting on the intent.
	c.Done(final)

	return nil
}

// computeDeletes creates a list of deletes to perform.  This will include any resources in the snapshot that were
// not encountered in the input, along with any resources that were replaced.
func (iter *PlanIterator) computeDeletes() []*resource.State {
	// To compute the deletion list, we must walk the list of old resources *backwards*.  This is because the list is
	// stored in dependency order, and earlier elements are possibly leaf nodes for later elements.  We must not delete
	// dependencies prior to their dependent nodes.
	var dels []*resource.State
	if prev := iter.p.prev; prev != nil {
		for i := len(prev.Resources) - 1; i >= 0; i-- {
			res := prev.Resources[i]
			urn := res.URN
			contract.Assert(!iter.creates[urn] || res.Delete)
			if res.Delete || (!iter.sames[urn] && !iter.updates[urn]) || iter.replaces[urn] {
				dels = append(dels, res)
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

	return NewSnapshot(iter.p.Target().Name, time.Now(), resources)
}

// CompleteStateSnapshot marks an old state snapshot as being processed, while adding a new version if relevant.  This
// is done to recover from failures partway through the application of a deployment plan.  Any old state that has not
// yet been recovered needs to be kept.
func (iter *PlanIterator) CompleteStateSnapshot(urn resource.URN, new *resource.State) {
	if old := iter.p.olds[urn]; old != nil {
		iter.dones[old] = true
	}
	if new != nil {
		iter.resources = append(iter.resources, new)
	}
}

// Provider fetches the provider for a given resource type, possibly lazily allocating the plugins for it.  If a
// provider could not be found, or an error occurred while creating it, a non-nil error is returned.
func (iter *PlanIterator) Provider(t tokens.Type) (plugin.Provider, error) {
	pkg := t.Package()
	prov, err := iter.p.ctx.Host.Provider(pkg)
	if err != nil {
		return nil, err
	} else if prov == nil {
		return nil, goerr.Errorf("could not load resource provider for package '%v' from $PATH", pkg)
	}
	return prov, nil
}
