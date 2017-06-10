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

package deployment

import (
	"github.com/golang/glog"
	goerr "github.com/pkg/errors"

	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/rendezvous"
)

// TODO[pulumi/lumi#106]: parallelism.

// Apply performs all steps in the plan, calling out to the progress reporting functions as desired.  It returns four
// things: the resulting Snapshot, no matter whether an error occurs or not; an error, if something went wrong; the step
// that failed, if the error is non-nil; and finally the state of the resource modified in the failing step.
func (p *Plan) Apply(prog Progress) (*Snapshot, *Step, resource.Status, error) {
	// Fetch a plan iterator and keep walking it until we are done.
	iter, err := p.Iterate()
	if err != nil {
		return nil, nil, resource.StatusOK, err
	}
	n := 1
	step, err := iter.Next()
	if err != nil {
		return nil, nil, resource.StatusOK, err
	}
	for step != nil {
		// Perform pre-application progress reporting.
		if prog != nil {
			prog.Before(step)
		}

		rst, err := step.Apply()

		// Perform post-application progress reporting.
		if prog != nil {
			prog.After(step, rst, err)
		}

		// If an error occurred, exit early.
		if err != nil {
			glog.V(7).Infof("Plan step #%v failed [%v]: %v", n, step.Op(), err)
			return iter.Snap(), step, rst, err
		}

		glog.V(7).Infof("Plan step #%v succeeded [%v]", n, step.Op())
		step, err = iter.Next()
		n++
	}

	// Finally, produce a new snapshot and return the resulting information.
	return iter.Snap(), nil, resource.StatusOK, nil
}

// Iterate initializes and returns an iterator that can be used to step through a plan's individual steps.
func (p *Plan) Iterate() (*PlanIterator, error) {
	// Manufacture an initial list of snapshots for the final output.
	var resources []*resource.State
	keeps := make(map[*resource.State]bool)
	for _, res := range p.Olds() {
		resources = append(resources, res)
		keeps[res] = true
	}

	// Start the planning process; this will perform initial steps that are required for the plan to be useable.
	rz := rendezvous.New()
	ps := &PlanIterator{
		p:         p,
		e:         eval.New(p.bindctx, newPlanEvalHooks(rz)),
		rz:        rz,
		creates:   make(map[resource.URN]bool),
		updates:   make(map[resource.URN]bool),
		deletes:   make(map[resource.URN]bool),
		sames:     make(map[resource.URN]bool),
		resources: resources,
		keeps:     keeps,
	}

	// First, populate the configuration variables.
	if err := ps.initConfig(); err != nil {
		return nil, err
	}

	// Give analyzers a chance to inspect the overall deployment.
	if err := ps.initAnalyzers(); err != nil {
		return nil, err
	}

	// Now create the evaluator coroutine and prepare it to take its first step.
	ps.forkEval()

	// If no error occurred, return a valid plan stepping object.
	return ps, nil
}

// PlanIterator can be used to step through and/or execute a plan's proposed actions.
type PlanIterator struct {
	p  *Plan                  // the plan to which this stepper belongs.
	e  eval.Interpreter       // the interpreter used to compute the new state.
	rz *rendezvous.Rendezvous // the rendezvous where planning and evaluator coroutines meet.

	creates  map[resource.URN]bool // URNs discovered to be created.
	updates  map[resource.URN]bool // URNs discovered to be updated.
	replaces map[resource.URN]bool // URNs discovered to be replaced.
	deletes  map[resource.URN]bool // URNs discovered to be deleted.
	sames    map[resource.URN]bool // URNs discovered to be the same.

	delqueue  []*resource.State        // a queue of deletes left to perform.
	resources []*resource.State        // the resulting ordered resource states.
	keeps     map[*resource.State]bool // a map of which states to keep in the final output.

	evaldone bool // true if the interpreter has been run to completion.
	done     bool // true if the planning and associated iteration has finished.
}

func (iter *PlanIterator) Plan() *Plan                    { return iter.p }
func (iter *PlanIterator) Interpreter() eval.Interpreter  { return iter.e }
func (iter *PlanIterator) Creates() map[resource.URN]bool { return iter.creates }
func (iter *PlanIterator) Updates() map[resource.URN]bool { return iter.updates }
func (iter *PlanIterator) Deletes() map[resource.URN]bool { return iter.deletes }
func (iter *PlanIterator) Sames() map[resource.URN]bool   { return iter.sames }
func (iter *PlanIterator) EvalDone() bool                 { return iter.evaldone }
func (iter *PlanIterator) Done() bool                     { return iter.done }

// initConfig applies the configuration map to an existing interpreter context.  The map is simply a map of tokens --
// which must be globally settable variables (module properties or static class properties) -- to serializable constant
// values.  The routine simply walks these tokens in sorted order, and assigns the constant objects.  Note that, because
// we are accessing module and class members, this routine will also trigger the relevant initialization routines.
func (iter *PlanIterator) initConfig() error {
	config := iter.p.config
	glog.V(5).Infof("Applying %v configuration values for package '%v'", len(config), iter.p.pkg)
	if config == nil {
		return nil
	}

	// For each config entry, bind the token to its symbol, and then attempt to assign to it.
	for _, tok := range config.Stable() {
		glog.V(5).Infof("Applying configuration value for token '%v'", tok)

		// Bind to the symbol; if it returns nil, this means an error has resulted, and we can skip it.
		var tree diag.Diagable // there is no source info for this; eventually we may assign one.
		if sym := iter.p.bindctx.LookupSymbol(tree, tokens.Token(tok), true); sym != nil {
			var ok bool
			switch s := sym.(type) {
			case *symbols.ModuleProperty:
				ok = true
			case *symbols.ClassProperty:
				// class properties are ok, so long as they are static.
				ok = s.Static()
			default:
				ok = false
			}
			if !ok {
				iter.p.Diag().Errorf(errors.ErrorIllegalConfigToken, tok)
				continue // skip to the next one
			}

			// Load up the location as an l-value; because we don't support instance properties, this is nil.
			if loc := iter.e.LoadLocation(tree, sym, nil, true); loc != nil {
				// Allocate a new constant for the value we are about to assign, and assign it to the location.
				v := config[tok]
				obj := rt.NewConstantObject(v)
				loc.Set(tree, obj)
			}
		}
	}

	if !iter.p.Diag().Success() {
		return goerr.New("Plan config application failed")
	}

	return nil
}

// initAnalyzers fires up the analyzers and asks them to validate the entire package.
func (iter *PlanIterator) initAnalyzers() error {
	for _, aname := range iter.p.analyzers {
		analyzer, err := iter.p.plugctx.Host.Analyzer(aname)
		if err != nil {
			return err
		}
		// TODO[pulumi/lumi#53]: we want to use the full package URL, including its SHA1 hash/version/etc.
		failures, err := analyzer.Analyze(pack.PackageURL{Name: iter.p.PkgName()})
		if err != nil {
			return err
		}
		for _, failure := range failures {
			iter.p.Diag().Errorf(errors.ErrorAnalyzeFailure, aname, failure.Reason)
		}
		if len(failures) > 0 {
			return goerr.New("At least one analyzer failured occurred")
		}
	}
	return nil
}

// forkEval performs the evaluation from a distinct goroutine.
func (iter *PlanIterator) forkEval() error {
	// Fire up the goroutine.
	go func() {
		iter.e.EvaluatePackage(iter.p.pkg, iter.p.args)
	}()

	// And wait for it to reach its rendezvous before proceeding.
	ret, done, err := iter.rz.Meet(planParty, nil)
	if err != nil {
		return err
	} else if done {
		contract.Assertf(!iter.p.Diag().Success(), "Expected diagnostics pertaining to the program failure")
		return goerr.New("Failure running the program before it even began executing")
	}
	contract.Assert(ret == nil)
	return nil
}

// Next advances the plan by a single step, and returns the next step to be performed.  In doing so, it will perform
// evaluation of the program as much as necessary to determine the next step.  If there is no further action to be
// taken, Next will return a nil step pointer.
func (iter *PlanIterator) Next() (*Step, error) {
	for !iter.done {
		if !iter.evaldone {
			// Kick the interpreter to compute some more and then inspect what it has to say.
			obj, done, err := iter.rz.Meet(planParty, nil)
			if err != nil {
				return nil, err
			} else if done {
				// If the interpreter is done, note it, and don't go back for more.
				iter.evaldone = true
				iter.delqueue = iter.calculateDeletes()
			} else {
				// Otherwise, check t see if there's something to be done for the given resource.
				contract.Assert(obj != nil)
				info := obj.(*AllocInfo)
				step, err := iter.nextResource(info)
				if err != nil {
					return nil, err
				} else if step != nil {
					return step, nil
				}

				// If the step returned was nil, this resource is fine, so we'll keep on going.
				continue
			}
		} else {
			// The interpreter has finished, so we need to now drain any deletions that piled up.
			if step := iter.nextDelete(); step != nil {
				return step, nil
			}

			// Otherwise, if the deletes have quiesced, there is nothing remaining in this plan; leave.
			iter.done = true
			break
		}
	}
	return nil, nil
}

// nextResource produces a new step for a given resource or nil if there isn't one to perform.
func (iter *PlanIterator) nextResource(info *AllocInfo) (*Step, error) {
	// Manufacture a resource object.
	obj := info.Obj
	new := resource.NewObject(obj)
	t := obj.Type().TypeToken()

	// Take a moment in time snapshot of the live object's properties.
	newprops := new.CopyProperties()

	// Fetch the provider for this resource type.
	prov, err := iter.p.Provider(new)
	if err != nil {
		return nil, err
	}

	// Fetch the resource's name from its provider, and use it to construct a URN.
	name, err := prov.Name(t, newprops)
	if err != nil {
		return nil, err
	}
	urn := resource.NewURN(iter.p.Namespace(), info.Mod.Tok, t, name)

	// Now see if there is an old resource and, if so, propagate the ID to the new object.
	var id resource.ID
	old, hasold := iter.p.Olds()[urn]
	if hasold {
		contract.Assert(old != nil && old.Type() == new.Type())
		id = old.ID()
		contract.Assert(id != "")
		new.SetID(id)                   // set the live object ID property.
		newprops = new.CopyProperties() // snap the properties again to reflect the ID.
	}

	// First ensure the provider is okay with this resource.
	var valid bool
	failures, err := prov.Check(new.Type(), newprops)
	if err != nil {
		return nil, err
	}
	for _, failure := range failures {
		valid = false
		iter.p.Diag().Errorf(
			errors.ErrorResourcePropertyInvalidValue, new.URN(), failure.Property, failure.Reason)
	}

	// Next, give each analyzer -- if any -- a chance to inspect the resource too.
	for _, a := range iter.p.analyzers {
		analyzer, err := iter.p.plugctx.Host.Analyzer(a)
		if err != nil {
			return nil, err
		}
		failures, err := analyzer.AnalyzeResource(t, newprops)
		if err != nil {
			return nil, err
		}
		for _, failure := range failures {
			valid = false
			iter.p.Diag().Errorf(
				errors.ErrorAnalyzeResourceFailure, a, new.URN(), failure.Property, failure.Reason)
		}
	}

	// If the resource isn't valid, don't proceed any further.
	if !valid {
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
		// The resource exists in both new and old; it could be an update.  This constitutes an update if the old
		// and new properties don't match exactly.  It is also possible we'll need to replace the resource if the
		// update impact assessment says so.  In this case, the resource's ID will change, which might have a
		// cascading impact on subsequent updates too, since those IDs must trigger recreations, etc.
		oldprops := old.Inputs()
		if oldprops.DeepEquals(newprops) {
			// No need to update anything, the properties didn't change; but we need to record the ID.
			iter.sames[urn] = true
			glog.V(7).Infof("Planner decided not to update '%v'", urn)
			return nil, nil
		}

		// The properties changed; we need to figure out whether to do an update or replacement.
		replacements, _, err := prov.InspectChange(t, id, oldprops, newprops)
		if err != nil {
			return nil, err
		} else if len(replacements) > 0 {
			iter.replaces[urn] = true
			glog.V(7).Infof("Planner decided to replace '%v'", urn)
			return NewReplaceStep(iter, old, new, newprops), nil
		}

		iter.updates[urn] = true
		glog.V(7).Infof("Planner decided to update '%v'", urn)
		return NewUpdateStep(iter, old, new, newprops), nil
	}

	// Otherwise, the resource isn't in the old map, so it must be a resource creation.
	iter.creates[urn] = true
	glog.V(7).Infof("Planner decided to create '%v'", urn)
	return NewCreateStep(iter, new, newprops), nil
}

// nextDelete produces a new step that deletes a resource if necessary.
func (iter *PlanIterator) nextDelete() *Step {
	if len(iter.delqueue) > 0 {
		del := iter.delqueue[0]
		iter.delqueue = iter.delqueue[1:]
		urn := del.URN()
		iter.deletes[urn] = true
		glog.V(7).Infof("Planner decided to delete '%v'", urn)
		return NewDeleteStep(iter, del)
	}
	return nil
}

// calculateDeletes creates a list of deletes to perform.  This will include any resources in the snapshot that were
// not encountered in the input, along with any resources that were replaced.
func (iter *PlanIterator) calculateDeletes() []*resource.State {
	var dels []*resource.State
	for _, res := range iter.p.old.Resources {
		urn := res.URN()
		contract.Assert(!iter.creates[urn])
		if iter.replaces[urn] || !iter.updates[urn] {
			dels = append(dels, res)
		}
	}
	return dels
}

// Snap returns a fresh snapshot that takes into account everything that has happend up till this point.  Namely, if a
// failure happens partway through, the untouched snapshot elements will be retained, while any updates will be
// preserved.  If no failure happens, the snapshot naturally reflects the final state of all resources.
func (iter *PlanIterator) Snap() *Snapshot {
	var resources []*resource.State
	for _, resource := range iter.resources {
		if iter.keeps[resource] {
			resources = append(resources, resource)
		}
	}
	return NewSnapshot(iter.p.old.Namespace, iter.p.old.Pkg, iter.p.args, resources)
}

// AppendStateSnapshot appends a resource's state to the current snapshot.
func (iter *PlanIterator) AppendStateSnapshot(state *resource.State) {
	iter.resources = append(iter.resources, state) // add this state to the pending list.
	iter.keeps[state] = true                       // ensure that we know to keep it in the final snapshot.
}

// RemoveStateSnapshot removes a resource's existing state from the current snapshot.
func (iter *PlanIterator) RemoveStateSnapshot(state *resource.State) {
	iter.keeps[state] = false // drop this state on the floor when creating the final snapshot.
}

// AllocInfo is the context in which an object got allocated.
type AllocInfo struct {
	Obj *rt.Object       // the object itself.
	Loc diag.Diagable    // the location information for the allocation.
	Pkg *symbols.Package // the package being evaluated when the allocation happened.
	Mod *symbols.Module  // the module being evaluated when the allocation happened.
	Fnc symbols.Function // the function being evaluated when the allocation happened.
}

// planEvalHooks are the interpreter hooks that synchronize between planner and evaluator in the appropriate ways.
type planEvalHooks struct {
	rz      *rendezvous.Rendezvous // the rendezvous object.
	currpkg *symbols.Package       // the current package being executed.
	currmod *symbols.Module        // the current module being executed.
	currfnc symbols.Function       // the current function being executed.
}

func newPlanEvalHooks(rz *rendezvous.Rendezvous) eval.Hooks {
	return &planEvalHooks{rz: rz}
}

// OnStart ensures that, before starting, we wait our turn.
func (h *planEvalHooks) OnStart() {
	ret, done, err := h.rz.Meet(evalParty, nil)
	contract.Assert(ret == nil)
	contract.Assertf(!done && err == nil, "Did not expect failure before even a single turn")
}

// OnDone ensures that, after completion, we tell the other party that we're done.
func (h *planEvalHooks) OnDone(uw *rt.Unwind) {
	h.rz.Done(nil)
}

// OnObjectInit ensures that, for every resource object created, we tell the planner about it.
func (h *planEvalHooks) OnObjectInit(tree diag.Diagable, obj *rt.Object) {
	if resource.IsResourceObject(obj) {
		// Communicate the full allocation context: AST node, package, module, and function.
		alloc := &AllocInfo{
			Obj: obj,
			Loc: tree,
			Pkg: h.currpkg,
			Mod: h.currmod,
			Fnc: h.currfnc,
		}
		ret, _, _ := h.rz.Meet(evalParty, alloc)
		contract.Assert(ret == nil)
		// TODO: if we are done, we need to inject an unwind or somesuch to stop the interpreter.
	}
}

// OnEnterPackage is invoked whenever we enter a new package.
func (h *planEvalHooks) OnEnterPackage(pkg *symbols.Package) func() {
	glog.V(9).Infof("GraphGenerator OnEnterPackage %v", pkg)
	prevpkg := h.currpkg
	h.currpkg = pkg
	return func() {
		glog.V(9).Infof("GraphGenerator OnLeavePackage %v", pkg)
		h.currpkg = prevpkg
	}
}

// OnEnterModule is invoked whenever we enter a new module.
func (h *planEvalHooks) OnEnterModule(mod *symbols.Module) func() {
	glog.V(9).Infof("GraphGenerator OnEnterModule %v", mod)
	prevmod := h.currmod
	h.currmod = mod
	return func() {
		glog.V(9).Infof("GraphGenerator OnLeaveModule %v", mod)
		h.currmod = prevmod
	}
}

// OnEnterFunction is invoked whenever we enter a new function.
func (h *planEvalHooks) OnEnterFunction(fnc symbols.Function) func() {
	glog.V(9).Infof("GraphGenerator OnEnterFunction %v", fnc)
	prevfnc := h.currfnc
	h.currfnc = fnc
	return func() {
		glog.V(9).Infof("GraphGenerator OnLeaveFunction %v", fnc)
		h.currfnc = prevfnc
	}
}
