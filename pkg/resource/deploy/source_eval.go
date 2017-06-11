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
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/rendezvous"
)

// NewEvalSource returns a planning source that fetches resources by evaluating a package pkg with a set of args args
// and a confgiuration map config.  This evaluation is performed using the given context ctx and may optionally use the
// given plugin host (or the default, if this is nil).  Note that closing the eval source also closes the host.
func NewEvalSource(ctx *binder.Context, pkg *symbols.Package, args core.Args, config resource.ConfigMap) Source {
	return &evalSource{
		bindctx: ctx,
		pkg:     pkg,
		args:    args,
		config:  config,
	}
}

type evalSource struct {
	bindctx *binder.Context    // the binder context (for compiler operations).
	pkg     *symbols.Package   // the package to evaluate.
	args    core.Args          // the arguments used to compile this package.
	config  resource.ConfigMap // the configuration variables for this package.
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
	if err := initEvalConfig(src, e); err != nil {
		return nil, err
	}

	// Now create the evaluator coroutine and prepare it to take its first step.
	forkEval(src, rz, e)

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
	rz  *rendezvous.Rendezvous // the rendezvous where planning and evaluator coroutines meet.
}

func (iter *evalSourceIterator) Close() error {
	// TODO: cancel the interpreter.
	iter.rz.Done(nil)
	return nil
}

func (iter *evalSourceIterator) Next() (*resource.Object, tokens.Module, error) {
	// Kick the interpreter to compute some more and then inspect what it has to say.
	obj, done, err := iter.rz.Meet(planParty, nil)
	if err != nil {
		return nil, "", err
	} else if done {
		glog.V(5).Infof("EvalSourceIterator is done")
		return nil, "", nil
	}

	// Otherwise, transform the object returned into a resource object that the planner can deal with.
	contract.Assert(obj != nil)
	info := obj.(*AllocInfo)
	glog.V(5).Infof("EvalSourceIterator produced a new object: obj=%v, ctx=%v", info.Obj, info.Mod.Tok)
	return resource.NewObject(info.Obj), info.Mod.Tok, nil
}

// initEvalConfig applies the configuration map to an existing interpreter context.  The map is simply a map of tokens --
// which must be globally settable variables (module properties or static class properties) -- to serializable constant
// values.  The routine simply walks these tokens in sorted order, and assigns the constant objects.  Note that, because
// we are accessing module and class members, this routine will also trigger the relevant initialization routines.
func initEvalConfig(src *evalSource, e eval.Interpreter) error {
	config := src.config
	glog.V(5).Infof("Applying %v configuration values for package '%v'", len(config), src.pkg)
	if config == nil {
		return nil
	}

	// For each config entry, bind the token to its symbol, and then attempt to assign to it.
	for _, tok := range config.StableKeys() {
		glog.V(5).Infof("Applying configuration value for token '%v'", tok)

		// Bind to the symbol; if it returns nil, this means an error has resulted, and we can skip it.
		var tree diag.Diagable // there is no source info for this; eventually we may assign one.
		if sym := src.bindctx.LookupSymbol(tree, tokens.Token(tok), true); sym != nil {
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
				src.bindctx.Diag.Errorf(errors.ErrorIllegalConfigToken, tok)
				continue // skip to the next one
			}

			// Load up the location as an l-value; because we don't support instance properties, this is nil.
			if loc := e.LoadLocation(tree, sym, nil, true); loc != nil {
				// Allocate a new constant for the value we are about to assign, and assign it to the location.
				v := config[tok]
				obj := rt.NewConstantObject(v)
				loc.Set(tree, obj)
			}
		}
	}

	return nil
}

// forkEval performs the evaluation from a distinct goroutine.  This function blocks until it's our turn to go.
func forkEval(src *evalSource, rz *rendezvous.Rendezvous, e eval.Interpreter) error {
	// Fire up the goroutine.
	go func() {
		e.EvaluatePackage(src.pkg, src.args)
	}()

	// And wait for it to reach its rendezvous before proceeding.
	ret, done, err := rz.Meet(planParty, nil)
	if err != nil {
		return err
	} else if done {
		return goerr.New("Failure running the program before it even began executing")
	}
	contract.Assert(ret == nil)
	return nil
}

// AllocInfo is the context in which an object got allocated.
type AllocInfo struct {
	Obj *rt.Object       // the object itself.
	Loc diag.Diagable    // the location information for the allocation.
	Pkg *symbols.Package // the package being evaluated when the allocation happened.
	Mod *symbols.Module  // the module being evaluated when the allocation happened.
	Fnc symbols.Function // the function being evaluated when the allocation happened.
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
	h.rz.Done(nil)
}

// OnObjectInit ensures that, for every resource object created, we tell the planner about it.
func (h *evalHooks) OnObjectInit(tree diag.Diagable, obj *rt.Object) {
	glog.V(9).Infof("EvalSource OnObjectInit %v (IsResource=%v)", obj, resource.IsResourceObject(obj))
	if resource.IsResourceObject(obj) {
		// Communicate the full allocation context: AST node, package, module, and function.
		alloc := &AllocInfo{
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

// OnEnterFunction is invoked whenever we enter a new function.
func (h *evalHooks) OnEnterFunction(fnc symbols.Function) func() {
	glog.V(9).Infof("EvalSource OnEnterFunction %v", fnc)
	prevfnc := h.currfnc
	h.currfnc = fnc
	return func() {
		glog.V(9).Infof("EvalSource OnLeaveFunction %v", fnc)
		h.currfnc = prevfnc
	}
}
