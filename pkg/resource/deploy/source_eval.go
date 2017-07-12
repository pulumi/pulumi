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

	"github.com/pulumi/lumi/pkg/compiler/binder"
	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types/predef"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/plugin"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/rendezvous"
)

// NewEvalSource returns a planning source that fetches resources by evaluating a package pkg with a set of args args
// and a confgiuration map config.  This evaluation is performed using the given context ctx and may optionally use the
// given plugin host (or the default, if this is nil).  Note that closing the eval source also closes the host.
//
// If destroy is true, then all of the usual initialization will take place, but the state will be presented to the
// planning engine as if no new resources exist.  This will cause it to forcibly remove them.
func NewEvalSource(plugctx *plugin.Context, bindctx *binder.Context,
	pkg *symbols.Package, args core.Args, config resource.ConfigMap, destroy bool) Source {
	return &evalSource{
		plugctx: plugctx,
		bindctx: bindctx,
		pkg:     pkg,
		args:    args,
		config:  config,
		destroy: destroy,
	}
}

type evalSource struct {
	plugctx *plugin.Context    // the plugin context (for plugin communication, e.g. interpreter state).
	bindctx *binder.Context    // the binder context (for compiler operations).
	pkg     *symbols.Package   // the package to evaluate.
	args    core.Args          // the arguments used to compile this package.
	config  resource.ConfigMap // the configuration variables for this package.
	destroy bool               // true if this source will trigger total destruction.
}

const (
	evalParty = rendezvous.PartyA // the evaluator's rendezvous party (it goes first).
	planParty = rendezvous.PartyB // the planner's rendezvous party (it goes second).
)

func (src *evalSource) Close() error {
	return nil
}

func (src *evalSource) Info() interface{} {
	return &evalSourceInfo{
		Pkg:  src.pkg.Tok,
		Args: src.args,
	}
}

// evalSourceInfo contains unique information about what source package plus arguments led to the resources.
type evalSourceInfo struct {
	Pkg  tokens.Package `json:"pkg"`
	Args core.Args      `json:"args"`
}

// Iterate will spawn an evaluator coroutine and prepare to interact with it on subsequent calls to Next.
func (src *evalSource) Iterate() (SourceIterator, error) {
	// Create a new rendezvous object used to orchestrate the planning and evaluation as coroutines.
	rz := rendezvous.New()

	// Now fire up a new interpreter.
	e := eval.New(src.bindctx, &evalHooks{rz: rz})

	// Populate the configuration variables.
	if err := InitEvalConfig(src.bindctx, e, src.config); err != nil {
		return nil, err
	}

	// Set the current context iterator; we will relinqush this in Close.
	src.plugctx.SetCurrentInterpreter(e)

	// Now create the evaluator coroutine and prepare it to take its first step.
	if err := forkEval(src, rz, e); err != nil {
		return nil, err
	}

	// Finally, return the fresh iterator that can take things from here.
	return &evalSourceIterator{
		src: src,
		e:   e,
		rz:  rz,
	}, nil
}

type evalSourceIterator struct {
	src *evalSource            // the owning eval source object.
	e   eval.Interpreter       // the interpreter used to compute the new state.
	res *resource.Object       // a resource to publish during the next rendezvous.
	rz  *rendezvous.Rendezvous // the rendezvous where planning and evaluator coroutines meet.
}

func (iter *evalSourceIterator) Close() error {
	// TODO: cancel the interpreter if it is still running.
	iter.rz.Done(nil)
	iter.src.plugctx.SetCurrentInterpreter(nil)
	return nil
}

func (iter *evalSourceIterator) Produce(res *resource.Object) {
	iter.res = res
}

func (iter *evalSourceIterator) Next() (*SourceAllocation, *SourceQuery, error) {
	// If we are destroying, we simply return nothing.
	if iter.src.destroy {
		return nil, nil, nil
	}

	// Kick the interpreter to compute some more and then inspect what it has to say.
	var data interface{}
	if res := iter.res; res != nil {
		data = rt.NewReturnUnwind(res.Obj())
		iter.res = nil // reset the state so we don't return things more than once.
	}
	obj, done, err := iter.rz.Meet(planParty, data)
	if err != nil {
		return nil, nil, err
	} else if done {
		glog.V(5).Infof("EvalSourceIterator is done")
		return nil, nil, nil
	}
	contract.Assert(obj != nil)

	// See what the interpreter came up with.  It's either an allocation or a query operation.
	if alloc, isalloc := obj.(*AllocRendezvous); isalloc {
		glog.V(5).Infof("EvalSourceIterator produced a new object: obj=%v, ctx=%v", alloc.Obj, alloc.Mod.Tok)
		return &SourceAllocation{
			Obj: resource.NewObject(alloc.Obj),
			Ctx: alloc.Mod.Tok,
		}, nil, nil
	} else if query, isquery := obj.(*QueryRendezvous); isquery {
		glog.V(5).Infof("EvalSourceIterator produced a new query: fnc=%v, #args=%v", query.Meth, len(query.Args))
		meth := query.Meth
		args := query.Args
		t := meth.Parent
		switch meth.Name() {
		case specialResourceGetFunction:
			if len(args) == 0 {
				return nil, nil,
					goerr.Errorf("Missing required argument 'id' for method %v", meth)
			} else if !args[0].IsString() {
				return nil, nil,
					goerr.Errorf("Expected method %v argument 'id' to be a string; got %v", meth, args[0])
			}
			return nil, &SourceQuery{Type: t, GetID: resource.ID(args[0].StringValue())}, nil
		case specialResourceQueryFunction: // Add similar checks to input args, if all good, then return nil, &SourceQuery{}, nil
			contract.Failf("TODO[pulumi/lumi#83]: query not yet implemented")
		default:
			contract.Failf("Unrecognized query rendezvous function name: %v", meth.Name())
		}
	}

	contract.Failf("Unexpected rendezvous object: %v (expected alloc or query)", obj)
	return nil, nil, nil
}

// InitEvalConfig applies the configuration map to an existing interpreter context.  The map is simply a map of tokens,
// which must be globally settable variables (module properties or static class properties), to serializable constant
// values.  The routine simply walks these tokens in sorted order, and assigns the constant objects.  Note that, because
// we are accessing module and class members, this routine will also trigger the relevant initialization routines.
func InitEvalConfig(ctx *binder.Context, e eval.Interpreter, config resource.ConfigMap) error {
	if config == nil {
		return nil
	}

	// For each config entry, bind the token to its symbol, and then attempt to assign to it.
	glog.V(5).Infof("Applying %v configuration values: %v", len(config), config)
	var err error
	for _, tok := range config.StableKeys() {
		glog.V(5).Infof("Applying configuration value for token '%v'", tok)

		// Bind to the symbol; if it returns nil, this means an error has resulted, and we can skip it.
		var tree diag.Diagable // there is no source info for this; eventually we may assign one.
		if sym := ctx.LookupSymbol(tree, tok, true); sym != nil {
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
				ctx.Diag.Errorf(errors.ErrorIllegalConfigToken, tok)
			} else {
				// Load up the location as an l-value; because we don't support instance properties, this is nil.
				loc, uw := e.LoadLocation(tree, sym, nil, true)
				if uw != nil {
					// If an error was thrown, print it and keep going.
					contract.Assert(uw.Throw())
					e.UnhandledException(tree, uw.Thrown())
					ok = false
				} else if loc != nil {
					// Allocate a new constant for the value we are about to assign, and assign it to the location.
					v := config[tok]
					obj := rt.NewConstantObject(v)
					loc.Set(tree, obj)
				}
			}
			if !ok && err == nil {
				err = goerr.New("Configuration variables could not be applied; stopping")
			}
		}
	}

	return err
}

// forkEval performs the evaluation from a distinct goroutine.  This function blocks until it's our turn to go.
func forkEval(src *evalSource, rz *rendezvous.Rendezvous, e eval.Interpreter) error {
	if src.destroy {
		// If we are destroying, no need to perform any evaluation beyond the config initialization.
	} else {
		// Fire up the goroutine.
		go func() {
			e.EvaluatePackage(src.pkg, src.args)
		}()

		// Let the other party run and only resume when it's our turn.
		ret, done, err := rz.Let(planParty)
		if err != nil {
			return err
		} else if done {
			return goerr.New("Failure running the program before it even began executing")
		}
		contract.Assertf(ret == nil, "unexpected rendezvous return: %v", ret)
	}

	return nil
}

// AllocRendezvous is used when an object is allocated, and tracks the context in which it was allocated.
type AllocRendezvous struct {
	Obj *rt.Object       // the object itself.
	Loc diag.Diagable    // the location information for the allocation.
	Pkg *symbols.Package // the package being evaluated when the allocation happened.
	Mod *symbols.Module  // the module being evaluated when the allocation happened.
	Fnc symbols.Function // the function being evaluated when the allocation happened.
}

// QueryRendezvous is used when the interpreter hits a query routine that needs to be evaluated by the planner.
type QueryRendezvous struct {
	Meth *symbols.ClassMethod // the resource method that triggered the need to rendezvous.
	Args []*rt.Object         // the arguments supplied, if any.
}

// evalHooks are the interpreter hooks that synchronize between planner and evaluator in the appropriate ways.
type evalHooks struct {
	rz      *rendezvous.Rendezvous // the rendezvous object.
	currpkg *symbols.Package       // the current package being executed.
	currmod *symbols.Module        // the current module being executed.
	currfnc symbols.Function       // the current function being executed.
}

// OnStart ensures that, before starting, we wait our turn.
func (h *evalHooks) OnStart() {
	ret, done, err := h.rz.Meet(evalParty, nil)
	contract.Assert(ret == nil)
	contract.Assertf(!done && err == nil, "Did not expect failure before even a single turn")
}

// OnDone ensures that, after completion, we tell the other party that we're done.
func (h *evalHooks) OnDone(uw *rt.Unwind) {
	var err error
	if uw != nil {
		if uw.Throw() {
			err = goerr.New("Planning resulted in an unhandled exception; cannot proceed with the plan")
		} else {
			contract.Assert(uw.Return())
		}
	}
	h.rz.Done(err)
}

// OnObjectInit ensures that, for every resource object created, we tell the planner about it.
func (h *evalHooks) OnObjectInit(tree diag.Diagable, obj *rt.Object) {
	glog.V(9).Infof("EvalSource OnObjectInit %v (IsResource=%v)", obj, resource.IsResourceObject(obj))
	if resource.IsResourceObject(obj) {
		// Communicate the full allocation context: AST node, package, module, and function.
		alloc := &AllocRendezvous{
			Obj: obj,
			Loc: tree,
			Pkg: h.currpkg,
			Mod: h.currmod,
			Fnc: h.currfnc,
		}
		ret, done, err := h.rz.Meet(evalParty, alloc)
		contract.Assert(ret == nil)
		contract.Assert(!done)
		contract.Assert(err == nil)
	}
}

// OnEnterPackage is invoked whenever we enter a new package.
func (h *evalHooks) OnEnterPackage(pkg *symbols.Package) func() {
	glog.V(9).Infof("EvalSource OnEnterPackage %v", pkg)
	prevpkg := h.currpkg
	h.currpkg = pkg
	return func() {
		glog.V(9).Infof("EvalSource OnLeavePackage %v", pkg)
		h.currpkg = prevpkg
	}
}

// OnEnterModule is invoked whenever we enter a new module.
func (h *evalHooks) OnEnterModule(mod *symbols.Module) func() {
	glog.V(9).Infof("EvalSource OnEnterModule %v", mod)
	prevmod := h.currmod
	h.currmod = mod
	return func() {
		glog.V(9).Infof("EvalSource OnLeaveModule %v", mod)
		h.currmod = prevmod
	}
}

const (
	specialResourceGetFunction   = "get"   // gets a single resource by ID.
	specialResourceQueryFunction = "query" // queries 0-to-many resources using arbitrary filters.
)

// OnEnterFunction is invoked whenever we enter a new function.  If it returns a non-nil unwind object, it will be used
// in place of the actual function call, effectively monkey patching it on the fly.
func (h *evalHooks) OnEnterFunction(fnc symbols.Function, args []*rt.Object) (*rt.Unwind, func()) {
	glog.V(9).Infof("EvalSource OnEnterFunction %v", fnc)
	prevfnc := h.currfnc
	h.currfnc = fnc

	// If this is one of the "special" resource functions, we need to essentially monkey patch it on the fly.
	var uw *rt.Unwind
	if meth, ismeth := fnc.(*symbols.ClassMethod); ismeth {
		if predef.IsResourceType(meth.Parent) {
			switch meth.Name() {
			case specialResourceGetFunction, specialResourceQueryFunction:
				// For any of these functions, we must defer to the planning side to do its thing.  After awaiting our
				// turn, we will be given an opportunity to resume with the object and/or unwind in hand.
				ret, done, err := h.rz.Meet(evalParty, &QueryRendezvous{
					Meth: meth,
					Args: args,
				})
				contract.Assertf(ret != nil, "Expecting unwind instructions from the planning goroutine")
				uw = ret.(*rt.Unwind)
				contract.Assert(!done)
				contract.Assert(err == nil)
			}
		}
	}

	return uw, func() {
		glog.V(9).Infof("EvalSource OnLeaveFunction %v", fnc)
		h.currfnc = prevfnc
	}
}
