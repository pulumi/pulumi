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

package eval

import (
	"math"
	"reflect"
	"sort"

	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/binder"
	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Interpreter can evaluate compiled LumiPacks.
type Interpreter interface {
	core.Phase

	Ctx() *binder.Context // the binding context object.

	// EvaluatePackage performs evaluation on the given blueprint package.
	EvaluatePackage(pkg *symbols.Package, args core.Args) (*rt.Object, *rt.Unwind)
	// EvaluateModule performs evaluation on the given module's entrypoint function.
	EvaluateModule(mod *symbols.Module, args core.Args) (*rt.Object, *rt.Unwind)
	// EvaluateFunction performs an evaluation of the given function, using the provided arguments.
	EvaluateFunction(fnc symbols.Function, this *rt.Object, args core.Args) (*rt.Object, *rt.Unwind)

	// LoadLocation loads a location by symbol; lval controls whether it is an l-value or just a value.
	LoadLocation(tree diag.Diagable, sym symbols.Symbol, this *rt.Object, lval bool) *Location
}

// New creates an interpreter that can be used to evaluate LumiPacks.
func New(ctx *binder.Context, hooks Hooks) Interpreter {
	e := &evaluator{
		ctx:        ctx,
		hooks:      hooks,
		modules:    make(moduleMap),
		statics:    make(staticMap),
		protos:     make(prototypeMap),
		modinits:   make(modinitMap),
		classinits: make(classinitMap),
	}
	newLocalScope(&e.locals, true, ctx.Scope)
	contract.Assert(e.locals != nil)
	return e
}

type evaluator struct {
	fnc        symbols.Function // the function currently under evaluation.
	ctx        *binder.Context  // the binding context with type and symbol information.
	hooks      Hooks            // callbacks that hook into interpreter events.
	modules    moduleMap        // the current module objects for all modules.
	statics    staticMap        // the object values for all static variable symbols.
	protos     prototypeMap     // the current "prototype" objects for all classes.
	stack      *rt.StackFrame   // a stack of frames to keep track of calls.
	locals     rt.Environment   // variable values corresponding to the current lexical scope.
	modinits   modinitMap       // a map of which modules have been initialized already.
	classinits classinitMap     // a map of which classes have been initialized already.
}

type moduleMap map[*symbols.Module]*rt.Object
type staticMap map[*symbols.Class]*rt.PropertyMap
type prototypeMap map[symbols.Type]*rt.Object
type modinitMap map[*symbols.Module]bool
type classinitMap map[*symbols.Class]bool

var _ Interpreter = (*evaluator)(nil)

func (e *evaluator) Ctx() *binder.Context { return e.ctx }
func (e *evaluator) Diag() diag.Sink      { return e.ctx.Diag }

// EvaluatePackage performs evaluation on the given blueprint package.
func (e *evaluator) EvaluatePackage(pkg *symbols.Package, args core.Args) (*rt.Object, *rt.Unwind) {
	glog.Infof("Evaluating package '%v'", pkg.Name())
	if e.hooks != nil {
		if leave := e.hooks.OnEnterPackage(pkg); leave != nil {
			defer leave()
		}
	}

	if glog.V(2) {
		defer glog.V(2).Infof("Evaluation of package '%v' completed w/ %v warnings and %v errors",
			pkg.Name(), e.Diag().Warnings(), e.Diag().Errors())
	}

	// Search the package for a default module to evaluate.
	var ret *rt.Object
	var uw *rt.Unwind
	defmod := pkg.Default()
	if defmod == nil {
		e.Diag().Errorf(errors.ErrorPackageHasNoDefaultModule.At(pkg.Tree()), pkg.Name())
	} else {
		ret, uw = e.EvaluateModule(defmod, args)
	}
	return ret, uw
}

// EvaluateModule performs evaluation on the given module's entrypoint function.
func (e *evaluator) EvaluateModule(mod *symbols.Module, args core.Args) (*rt.Object, *rt.Unwind) {
	glog.Infof("Evaluating module '%v'", mod.Token())
	if e.hooks != nil {
		if leave := e.hooks.OnEnterModule(mod); leave != nil {
			defer leave()
		}
	}

	if glog.V(2) {
		defer glog.V(2).Infof("Evaluation of module '%v' completed w/ %v warnings and %v errors",
			mod.Token(), e.Diag().Warnings(), e.Diag().Errors())
	}

	// Fetch the module's entrypoint function, erroring out if it doesn't have one.
	var ret *rt.Object
	var uw *rt.Unwind
	hadEntry := false
	if entry, has := mod.Members[tokens.EntryPointFunction]; has {
		if entryfnc, ok := entry.(symbols.Function); ok {
			ret, uw = e.EvaluateFunction(entryfnc, nil, args)
			hadEntry = true
		}
	}

	if !hadEntry {
		e.Diag().Errorf(errors.ErrorModuleHasNoEntryPoint.At(mod.Tree()), mod.Name())
	}

	return ret, uw
}

// EvaluateFunction performs an evaluation of the given function, using the provided arguments.
func (e *evaluator) EvaluateFunction(fnc symbols.Function, this *rt.Object, args core.Args) (*rt.Object, *rt.Unwind) {
	glog.Infof("Evaluating function '%v'", fnc.Token())
	if glog.V(2) {
		defer glog.V(2).Infof("Evaluation of function '%v' completed w/ %v warnings and %v errors",
			fnc.Token(), e.Diag().Warnings(), e.Diag().Errors())
	}

	// Call the pre-start hook if registered.
	if e.hooks != nil {
		e.hooks.OnStart()
	}

	// Ensure all exit paths do the right thing (dumping, exceptions, hooks).
	var uw *rt.Unwind
	defer (func() {
		// Dump the evaluation state at log-level 5, if it is enabled.
		e.dumpEvalState(5)

		// If the call had a throw unwind, then we have an unhandled exception.
		if uw != nil && uw.Throw() {
			e.issueUnhandledException(uw, errors.ErrorUnhandledException.At(fnc.Tree()))
		}

		// Make sure to invoke the done hook.
		if e.hooks != nil {
			e.hooks.OnDone(uw)
		}
	})()

	// Ensure that initializers have been run.
	switch f := fnc.(type) {
	case *symbols.ClassMethod:
		if uw = e.ensureClassInit(f.Parent); uw != nil {
			return nil, uw
		}
	case *symbols.ModuleMethod:
		if uw = e.ensureModuleInit(f.Parent); uw != nil {
			return nil, uw
		}
	default:
		contract.Failf("Unrecognized function evaluation type: %v", reflect.TypeOf(f))
	}

	// First, validate any arguments, and turn them into real runtime *rt.Objects.
	var argos []*rt.Object
	params := fnc.Function().GetParameters()
	if params == nil {
		if len(args) != 0 {
			e.Diag().Errorf(errors.ErrorFunctionArgMismatch.At(fnc.Tree()), 0, len(args))
		}
	} else {
		if len(*params) != len(args) {
			e.Diag().Errorf(errors.ErrorFunctionArgMismatch.At(fnc.Tree()), 0, len(args))
		}

		ptys := fnc.Signature().Parameters
		found := make(map[tokens.Name]bool)
		for i, param := range *params {
			pname := param.Name.Ident
			if arg, has := args[pname]; has {
				found[pname] = true
				argo := rt.NewConstantObject(arg)
				if !types.CanConvert(argo.Type(), ptys[i]) {
					e.Diag().Errorf(errors.ErrorFunctionArgIncorrectType.At(fnc.Tree()), ptys[i], argo.Type())
					break
				}
				argos = append(argos, argo)
			} else {
				e.Diag().Errorf(errors.ErrorFunctionArgNotFound.At(fnc.Tree()), param.Name)
			}
		}
		for arg := range args {
			if !found[arg] {
				e.Diag().Errorf(errors.ErrorFunctionArgUnknown.At(fnc.Tree()), arg)
			}
		}
	}

	// If the arguments bound correctly, actually perform the invocation.
	var ret *rt.Object
	if e.Diag().Success() {
		ret, uw = e.evalCallSymbol(fnc.Tree(), fnc, this, argos...)
	}
	return ret, uw
}

// Utility functions

// dumpEvalState logs the evaluator's current state at the given log-level.
func (e *evaluator) dumpEvalState(v glog.Level) {
	if glog.V(v) {
		glog.V(v).Infof("Evaluator state dump:")
		glog.V(v).Infof("=====================")

		// Print all initialized modules in alphabetical order.
		modtoks := make([]string, 0, len(e.modinits))
		for mod := range e.modinits {
			modtoks = append(modtoks, string(mod.Token()))
		}
		sort.Strings(modtoks)
		for _, mod := range modtoks {
			glog.V(v).Infof("Module init: %v", mod)
		}

		// Print all initialized classes in alphabetical order.
		classtoks := make([]string, 0, len(e.classinits))
		for class := range e.classinits {
			classtoks = append(classtoks, string(class.Token()))
		}
		sort.Strings(classtoks)
		for _, class := range classtoks {
			glog.V(v).Infof("Class init: %v", class)
		}
	}
}

// initProperty initializes a property entry in the given map, using an optional `this` pointer for member functions.
// It returns the resulting pointer along with a boolean to indicate whether the property was left unfrozen.
func (e *evaluator) initProperty(this *rt.Object, properties *rt.PropertyMap,
	key rt.PropertyKey, sym symbols.Symbol) (*rt.Pointer, bool) {
	// First, ensure we've swapped in an intrinsic if available.
	contract.Assert(sym != nil)
	sym = MaybeIntrinsic(sym)

	switch m := sym.(type) {
	case symbols.Function:
		// A function results in a closure object referring to `this`, if any.
		obj := rt.NewFunctionObjectFromSymbol(m, this)
		// TODO[pulumi/lumi#56]: all methods are readonly; consider permitting JS-style overwriting of them.
		return properties.InitAddr(key, obj, true, nil, nil), false
	case symbols.Variable:
		// A variable could have a default object; if so, use that; otherwise, null will be substituted automatically.
		var obj *rt.Object
		if m.Default() != nil {
			// If the variable has a default object, substitute it.
			obj = rt.NewConstantObject(*m.Default())
		} else if this != nil && m.Computed() {
			// If the variable is a computed one -- that is, assignments may come from outside the system at a later
			// date and we want to be able to speculate beyond it -- then stick a computed value in the slot.
			obj = rt.NewComputedObject(m.Type(), false, []*rt.Object{this})
		}

		// If this is a class property, it may have a getter and/or setter; flow them.
		var get symbols.Function
		var set symbols.Function
		if prop, isprop := m.(*symbols.ClassProperty); isprop {
			get = prop.Getter()
			set = prop.Setter()
		}
		contract.Assertf(obj == nil || (get == nil && set == nil), "Inits, getters, and setters cannot be mixed")

		// Finally allocate the slot and return the resulting pointer to that slot.
		ptr := properties.InitAddr(key, obj, false, get, set)
		return ptr, m.Readonly()
	case *symbols.Module:
		// A module resolves to its module object.
		modobj := e.getModuleObject(m)
		return properties.InitAddr(key, modobj, false, nil, nil), false
	case *symbols.Class:
		// A class resolves to its prototype object.
		proto := e.getPrototypeObject(m)
		return properties.InitAddr(key, proto, false, nil, nil), false
	default:
		contract.Failf("Unrecognized property '%v' symbol type: %v", key, reflect.TypeOf(sym))
		return nil, false
	}
}

// ensureClassInit ensures that the target's class initializer has been run.
func (e *evaluator) ensureClassInit(class *symbols.Class) *rt.Unwind {
	already := e.classinits[class]
	e.classinits[class] = true // set true before running, in case of cycles.

	if !already {
		// First ensure the module initializer has run.
		if uw := e.ensureModuleInit(class.Parent); uw != nil {
			return uw
		}

		// Next ensure that the base class's statics are initialized.
		if class.Extends != nil {
			if extends, isclass := class.Extends.(*symbols.Class); isclass {
				e.ensureClassInit(extends)
			}
		}

		// Now populate this class's statics with all of the static members.
		var readonlines []*rt.Pointer
		statics := e.getClassStatics(class)
		var current symbols.Type = class
		for current != nil {
			members := current.TypeMembers()
			for _, name := range symbols.StableClassMemberMap(members) {
				if member := members[name]; member.Static() {
					key := rt.PropertyKey(name)
					if ptr, readonly := e.initProperty(nil, statics, key, member); readonly {
						// Readonly properties are unfrozen during initialization; afterwards, they will be frozen.
						readonlines = append(readonlines, ptr)
					}
				}
			}

			// Keep going up the type hierarchy.
			current = current.Base()
		}

		// Next, run the class if it has an initializer.
		if init := class.GetInit(); init != nil {
			glog.V(7).Infof("Initializing class: %v", class)
			contract.Assert(len(init.Sig.Parameters) == 0)
			contract.Assert(init.Sig.Return == nil)
			ret, uw := e.evalCallSymbol(class.Tree(), init, nil)
			if uw != nil {
				return uw
			}
			contract.Assert(ret == nil)
		} else {
			glog.V(7).Infof("Class has no initializer: %v", class)
		}

		// Now, finally, ensure that all readonly class statics are frozen.
		for _, readonly := range readonlines {
			readonly.Freeze() // ensure this cannot be written post-initialization.
		}
	}

	return nil
}

// ensureModuleInit ensures that the target's module initializer has been run.  It also evaluates dependency module
// initializers, assuming they have been declared.  If they have not, those will run when we access them.
func (e *evaluator) ensureModuleInit(mod *symbols.Module) *rt.Unwind {
	already := e.modinits[mod]
	e.modinits[mod] = true // set true before running, in case of cycles.

	if !already {
		// Populate all properties in this module, even if they will be empty for now.
		var readonlines []*rt.Pointer
		globals := e.getModuleGlobals(mod)
		for _, name := range symbols.StableModuleMemberMap(mod.Members) {
			key := rt.PropertyKey(name)
			member := mod.Members[name]
			if ptr, readonly := e.initProperty(nil, globals.Properties(), key, member); readonly {
				// If this property was left unfrozen, be sure to remember it for freezing after we're done.
				readonlines = append(readonlines, ptr)
			}
		}

		// Next, run the module initializer if it has one.
		if init := mod.GetInit(); init != nil {
			glog.V(7).Infof("Initializing module: %v", mod)
			contract.Assert(len(init.Sig.Parameters) == 0)
			contract.Assert(init.Sig.Return == nil)
			ret, uw := e.evalCallSymbol(mod.Tree(), init, nil)
			if uw != nil {
				return uw
			}
			contract.Assert(ret == nil)
		} else {
			glog.V(7).Infof("Module has no initializer: %v", mod)
		}

		// Ensure that all readonly module properties are frozen.
		for _, readonly := range readonlines {
			readonly.Freeze() // ensure this is never written to after initialization.
		}
	}

	return nil
}

// getModuleGlobals returns a globals table for the given module, lazily initializing if needed.
func (e *evaluator) getModuleGlobals(module *symbols.Module) *rt.Object {
	// If there's an existing globals object, return it.
	if globals, has := e.modules[module]; has {
		return globals
	}

	// Otherwise, we need to create a fresh module object.  This is laid out in a very specific way to facilitate
	// information hiding.  There are two objects: 1) the super (prototype) object, containing the full set of exported
	// variables, facilitating dynamic access; and 2) the child object, whose map contains all the globals, etc. that
	// are not exported for use outside of the module itself.  We access one or the other as appropriate.

	// First, create the super object, with all of the exports.
	proto := rt.NewObject(symbols.NewModuleType(module), nil, nil, nil)
	props := proto.Properties()
	for _, name := range symbols.StableModuleExportMap(module.Exports) {
		key := rt.PropertyKey(name)
		if props.GetAddr(key) == nil {
			exp := module.Exports[name]
			e.initProperty(proto, props, key, e.chaseExports(exp))
		}
	}

	// Next, create the childmost object and link its parent to the proto.
	globals := rt.NewObject(symbols.NewModuleType(module), nil, nil, proto)

	// Memoize the result and return it.
	e.modules[module] = globals
	return globals
}

// getClassStatics returns a statics table for the given class, lazily initializing if needed.
func (e *evaluator) getClassStatics(class *symbols.Class) *rt.PropertyMap {
	statics, has := e.statics[class]
	if !has {
		// TODO[pulumi/lumi#176]: merge the class statics representation with the prototype object representation.
		statics = rt.NewPropertyMap()
		e.statics[class] = statics
	}
	return statics
}

// getModuleObject returns a module object for the given module.  This object contains all exported members through
// properties.  This is a mutable object, and so is cached and reused for subsequent lookups.
func (e *evaluator) getModuleObject(m *symbols.Module) *rt.Object {
	// To get the module object, simply fetch the globals.  The super proto class of the globals will contain only the
	// exported variables, which is suitable for returning to code on the import side.
	globals := e.getModuleGlobals(m)
	contract.Assert(globals != nil)
	contract.Assert(globals.Proto() != nil)
	return globals.Proto()
}

// getPrototypeObject returns the prototype for a given type.  The prototype is a mutable object, and so it is cached,
// and reused for subsequent lookups.  This means that mutations in the prototype are lasting and visible for all later
// uses.  This is similar to ECMAScript behavior; see http://www.ecma-international.org/ecma-262/6.0/#sec-objects.
// TODO[pulumi/lumi#70]: technically this should be gotten from the constructor function object; we will need to
//     rewire things a bit, depending on how serious we are about ECMAScript compliance, especially dynamic scenarios.
func (e *evaluator) getPrototypeObject(t symbols.Type) *rt.Object {
	// If there is already a proto for this type, use it.
	if proto, has := e.protos[t]; has {
		return proto
	}

	// If not, we need to create a new one.  First, fetch the base if there is one.
	var base *rt.Object
	if t.Base() != nil {
		base = e.getPrototypeObject(t.Base())
	}

	// Now populate the prototype object with all members.
	proto := rt.NewObject(symbols.NewPrototypeType(t), nil, nil, base)
	e.addPrototypeObjectMembers(proto, t.TypeMembers(), true)

	// For any interfaces implemented by the type, type also ensure to add these as though they were members.
	if class, isclass := t.(*symbols.Class); isclass {
		for _, impl := range class.Implements {
			e.addPrototypeObjectInterfaceMembers(proto, impl)
		}
	}

	e.protos[t] = proto
	return proto
}

// addPrototypeObjectMembers adds a bag of members to a prototype (during initialization).
func (e *evaluator) addPrototypeObjectMembers(proto *rt.Object, members symbols.ClassMemberMap, must bool) {
	properties := proto.Properties()
	for _, name := range symbols.StableClassMemberMap(members) {
		if member := members[name]; !member.Static() {
			key := rt.PropertyKey(name)
			if must || properties.GetAddr(key) == nil {
				e.initProperty(proto, properties, key, member)
			}
		}
	}
}

// addPrototypeObjectInterfaceMembers ensures that interface members exist on the prototype (during initialization).
func (e *evaluator) addPrototypeObjectInterfaceMembers(proto *rt.Object, impl symbols.Type) {
	// Add members from this type, but only so long as they don't already exist.
	e.addPrototypeObjectMembers(proto, impl.TypeMembers(), false)

	// Now do the same for any base classes.
	if base := impl.Base(); base != nil {
		e.addPrototypeObjectInterfaceMembers(proto, base)
	}
	if class, isclass := impl.(*symbols.Class); isclass {
		for _, classimpl := range class.Implements {
			e.addPrototypeObjectInterfaceMembers(proto, classimpl)
		}
	}
}

// newObject allocates a fresh object of the given type, wired up to its prototype.
func (e *evaluator) newObject(t symbols.Type) *rt.Object {
	// First, fetch the prototype chain for this object.  This is required to implement property chaining.
	proto := e.getPrototypeObject(t)

	// Now create an empty object of the desired type.  Subsequent operations will do the right thing with it.  E.g.,
	// overwriting a property will add a new entry to the object's map; reading will search the prototpe chain; etc.
	return rt.NewObject(t, nil, nil, proto)
}

// issueUnhandledException issues an unhandled exception error using the given diagnostic and unwind information.
func (e *evaluator) issueUnhandledException(uw *rt.Unwind, err *diag.Diag, args ...interface{}) {
	contract.Assert(uw.Throw())

	// Produce a message with the exception text plus stack trace.
	var msg string
	if ex := uw.Exception(); ex != nil {
		if ex.Thrown.Type() == types.String {
			msg = ex.Thrown.StringValue() // use the basic string value.
		} else {
			msg = "\n" + ex.Thrown.Details(false, "\t") // convert the thrown object into a detailed string
		}
		msg += "\n" + ex.Stack.Trace(e.Diag(), "\t", ex.Node)
	} else {
		msg = "no details available"
	}

	// Now simply output the error with the message plus stack trace.
	args = append(args, msg)
	e.Diag().Errorf(err, args...)
}

// rejectComputed checks an object's value and, if it's computed and its value isn't known, returns an exception unwind.
func (e *evaluator) rejectComputed(tree diag.Diagable, obj *rt.Object) *rt.Unwind {
	if obj != nil && obj.Type().Computed() {
		// TODO[pulumi/lumi#170]: support multi-stage planning that speculates beyond conditionals.
		return e.NewUnexpectedComputedValueException(tree, obj)
	}
	return nil
}

// pushModuleScope establishes a new module-wide scope.  It returns a function that restores the prior value.
func (e *evaluator) pushModuleScope(m *symbols.Module) func() {
	return e.ctx.PushModule(m)
}

// pushClassScope establishes a new class-wide scope.  This also establishes the right module context.  If the object
// argument is non-nil, instance methods are also populated.  It returns a function that restores the prior value.
func (e *evaluator) pushClassScope(c *symbols.Class, obj *rt.Object) func() {
	contract.Assert(obj == nil || types.CanConvert(obj.Type(), c))
	return e.ctx.PushClass(c)
}

// pushScope pushes a new local and context scope.  The activation argument indicates whether this is an activation
// frame, meaning that searches for local variables will not probe into parent scopes (since they are inaccessible).
func (e *evaluator) pushScope(frame *rt.StackFrame, activation bool) {
	if frame != nil {
		frame.Parent = e.stack // remember the parent so we can pop.
		e.stack = frame        // install this as the current frame.
	}
	e.locals.Push(activation) // pushing the local scope also updates the context scope.
}

// popScope pops the current local and context scopes.
func (e *evaluator) popScope(frame *rt.StackFrame) {
	if frame != nil {
		contract.Assert(e.stack == frame)
		e.stack = e.stack.Parent
	}
	e.locals.Pop() // popping the local scope also updates the context scope.
}

// Functions

func (e *evaluator) evalCall(node diag.Diagable,
	fnc ast.Function, sym symbols.Function, sig *symbols.FunctionType, shareActivation bool,
	this *rt.Object, args ...*rt.Object) (*rt.Object, *rt.Unwind) {
	var label string
	if sym == nil {
		label = "lambda"
	} else {
		label = string(sym.Token())
	}
	glog.V(7).Infof("Evaluating call to fnc %v; this=%v args=%v", label, this != nil, len(args))

	// First check the this pointer, since it might throw before the call even happens.
	intrinsic := false
	var thisVariable *symbols.LocalVariable
	var superVariable *symbols.LocalVariable
	if sym != nil {
		for fsym, done := sym, false; !done; {
			contract.Assert(fsym != nil)
			// Set up the appropriate this/super variables, and also ensure that we enter the right module/class
			// context (otherwise module-sensitive binding won't work).
			switch f := fsym.(type) {
			case *symbols.ClassMethod:
				if f.Static() {
					if this != nil {
						// A non-nil `this` is okay if we loaded this function from a prototype object.
						prototy, isproto := this.Type().(*symbols.PrototypeType)
						contract.Assert(isproto)
						contract.Assert(prototy.Type == f.Parent)
					}
				} else {
					contract.Assertf(this != nil, "Expect non-nil this to invoke '%v'", f)
					if uw := e.checkThis(node, this); uw != nil {
						return nil, uw
					}
					thisVariable = f.Parent.This
					superVariable = f.Parent.Super
				}

				popModule := e.pushModuleScope(f.Parent.Parent)
				defer popModule()
				popClass := e.pushClassScope(f.Parent, this)
				defer popClass()
				done = true

			case *symbols.ModuleMethod:
				if this != nil {
					// A non-nil `this` is okay if we loaded this function from a module object.  Because modules can
					// re-export members from other modules, we cannot require that the type's parent matches.
					_, ismod := this.Type().(*symbols.ModuleType)
					contract.Assert(ismod)
					this = nil // the this parameter isn't required during invocation.
				}

				popModule := e.pushModuleScope(f.Parent)
				defer popModule()
				done = true

			case *rt.Intrinsic:
				intrinsic = true
				fsym = f.UnderlyingSymbol() // swap in the underlying symbol for purposes of this/super/scoping.
				if fsym == nil {            // Builtin intrinsics may not have an underlying symbol.
					done = true
				}

			default:
				contract.Failf("Unrecognized function type during call: %v", reflect.TypeOf(fsym))
			}
		}
	}

	// Save the prior func symbol, set the new one, and restore upon exit.
	prior := sym
	e.fnc = sym
	defer func() { e.fnc = prior }()

	// Set up a new lexical scope "activation frame" in which we can bind the parameters; restore it upon exit.
	frame := &rt.StackFrame{Node: fnc, Func: sym, Caller: node}
	e.pushScope(frame, !shareActivation)
	defer e.popScope(frame)

	// Invoke the hooks if available.
	if e.hooks != nil {
		if leave := e.hooks.OnEnterFunction(sym); leave != nil {
			defer leave()
		}
	}

	// If the target is an instance method, the "this" and "super" variables must be bound to values.
	if thisVariable != nil {
		contract.Assert(this != nil)
		e.locals.Register(thisVariable)
		e.locals.InitValueAddr(thisVariable, rt.NewPointer(this, true, nil, nil))
	}
	if superVariable != nil {
		contract.Assert(this != nil)
		e.locals.Register(superVariable)
		e.locals.InitValueAddr(superVariable, rt.NewPointer(this, true, nil, nil))
	}

	// Ensure that the arguments line up to the parameter slots and add them to the frame.
	if fnc != nil {
		params := fnc.GetParameters()
		if params == nil {
			contract.Assert(len(args) == 0)
		} else {
			contract.Assertf(len(args) == len(*params),
				"Expected argc %v == paramc %v in call to '%v'", len(args), len(*params), label)
			for i, param := range *params {
				sym := e.ctx.RequireVariable(param).(*symbols.LocalVariable)
				e.locals.Register(sym)
				arg := args[i]
				contract.Assert(types.CanConvert(arg.Type(), sym.Type()))
				e.locals.SetValue(sym, arg)
			}
		}
	}

	// Now perform the invocation; for intrinsics, just run the code; for all others, interpret the body.
	var uw *rt.Unwind
	if intrinsic {
		isym := sym.(*rt.Intrinsic)
		invoker := GetIntrinsicInvoker(isym)
		uw = invoker(isym, e, this, args)
	} else {
		uw = e.evalStatement(fnc.GetBody())
	}

	// Check that the unwind is as expected.  In particular:
	//     1) no breaks or continues are expected;
	//     2) any throw is treated as an unhandled exception that propagates to the caller.
	//     3) any return is checked to be of the expected type, and returned as the result of the call.
	retty := sig.Return
	if uw != nil {
		if uw.Throw() {
			if glog.V(7) {
				glog.V(7).Infof("Evaluated call to fnc %v; unhandled exception: %v", fnc, uw.Exception().Thrown)
			}
			return nil, uw
		}

		contract.Assert(uw.Return()) // break/continue not expected.
		ret := uw.Returned()
		contract.Assert((retty == nil) == (ret == nil))
		contract.Assert(ret == nil || types.CanConvert(ret.Type(), retty))
		if glog.V(7) {
			glog.V(7).Infof("Evaluated call to fnc %v; return=%v", label, ret)
		}
		return ret, nil
	}

	// An absence of a return is okay for void- or dynamic-returning functions.
	contract.Assert(retty == nil || retty == types.Dynamic)
	glog.V(7).Infof("Evaluated call to fnc %v; return=<nil>", label)
	return nil, nil
}

func (e *evaluator) evalCallSymbol(node diag.Diagable, fnc symbols.Function,
	this *rt.Object, args ...*rt.Object) (*rt.Object, *rt.Unwind) {
	return e.evalCall(node, fnc.Function(), fnc, fnc.Signature(), false, this, args...)
}

func (e *evaluator) evalCallFuncStub(node diag.Diagable,
	fnc rt.FuncStub, args ...*rt.Object) (*rt.Object, *rt.Unwind) {
	// If there is an environment for the call stub, restore it before invoking and then put it back.
	var shareActivation bool
	if fnc.Env != nil {
		restore := e.locals.Swap(fnc.Env)
		defer restore()
		shareActivation = true
	}
	return e.evalCall(node, fnc.Func, fnc.Sym, fnc.Sig, shareActivation, fnc.This, args...)
}

// Statements

func (e *evaluator) evalStatement(node ast.Statement) *rt.Unwind {
	contract.Assert(node != nil)
	if glog.V(7) {
		glog.V(7).Infof("Evaluating statement: %v", reflect.TypeOf(node))
	}

	// Simply switch on the node type and dispatch to the specific function, returning the rt.Unwind info.
	switch n := node.(type) {
	case *ast.Import:
		return e.evalImport(n)
	case *ast.Block:
		return e.evalBlock(n)
	case *ast.LocalVariableDeclaration:
		return e.evalLocalVariableDeclaration(n)
	case *ast.TryCatchFinally:
		return e.evalTryCatchFinally(n)
	case *ast.BreakStatement:
		return e.evalBreakStatement(n)
	case *ast.ContinueStatement:
		return e.evalContinueStatement(n)
	case *ast.IfStatement:
		return e.evalIfStatement(n)
	case *ast.SwitchStatement:
		return e.evalSwitchStatement(n)
	case *ast.LabeledStatement:
		return e.evalLabeledStatement(n)
	case *ast.ReturnStatement:
		return e.evalReturnStatement(n)
	case *ast.ThrowStatement:
		return e.evalThrowStatement(n)
	case *ast.WhileStatement:
		return e.evalWhileStatement(n)
	case *ast.ForStatement:
		return e.evalForStatement(n)
	case *ast.EmptyStatement:
		return nil // nothing to do
	case *ast.MultiStatement:
		return e.evalMultiStatement(n)
	case *ast.ExpressionStatement:
		return e.evalExpressionStatement(n)
	default:
		contract.Failf("Unrecognized statement node kind: %v", node.GetKind())
		return nil
	}
}

func (e *evaluator) evalImport(node *ast.Import) *rt.Unwind {
	// Ensure the target module has been initialized.
	imptok := node.Referent
	sym := e.ctx.LookupSymbol(imptok, imptok.Tok, true)
	contract.Assert(sym != nil)

	var mod *symbols.Module
	switch s := sym.(type) {
	case *symbols.Module:
		mod = s
	case symbols.ModuleMember:
		mod = s.MemberParent()
	default:
		contract.Failf("Unrecognized import symbol: %v", reflect.TypeOf(sym))
	}
	if uw := e.ensureModuleInit(mod); uw != nil {
		return uw
	}

	// If a name was requested, bind it dynamically to an object with this import's exports.
	// TODO[pulumi/lumi#176]: a more elegant way to do this might be to bind it statically, however that's complex
	//     because then it would potentially require a module property versus local variable distinction.
	if node.Name != nil {
		key := tokens.Name(node.Name.Ident)
		addr := e.getDynamicNameAddr(key, true)
		modobj := e.getModuleObject(mod)
		addr.Set(modobj)
	}

	return nil
}

func (e *evaluator) evalBlock(node *ast.Block) *rt.Unwind {
	// Push a scope at the start, and pop it at afterwards; both for the symbol context and local variable values.
	e.pushScope(nil, false)
	defer e.popScope(nil)

	for _, stmt := range node.Statements {
		if uw := e.evalStatement(stmt); uw != nil {
			return uw
		}
	}

	return nil
}

func (e *evaluator) evalLocalVariableDeclaration(node *ast.LocalVariableDeclaration) *rt.Unwind {
	// Populate the variable in the scope.
	loc := node.Local
	sym := e.ctx.RequireVariable(loc).(*symbols.LocalVariable)
	e.locals.Register(sym)

	// If there is a default value, set it now.
	if loc.Default != nil {
		obj := rt.NewConstantObject(*loc.Default)
		e.locals.SetValue(sym, obj)
	}

	return nil
}

func (e *evaluator) evalTryCatchFinally(node *ast.TryCatchFinally) *rt.Unwind {
	// First, execute the try part.
	uw := e.evalStatement(node.TryClause)
	if uw != nil && uw.Throw() {
		// The try block threw something; see if there is a handler that covers this.
		thrown := uw.Exception().Thrown
		if node.CatchClauses != nil {
			for _, catch := range *node.CatchClauses {
				ex := e.ctx.RequireVariable(catch.Exception).(*symbols.LocalVariable)
				exty := ex.Type()
				if types.CanConvert(thrown.Type(), exty) {
					// This type matched, so this handler will catch the exception.  Set the exception variable,
					// evaluate the block, and swap the rt.Unwind information ("handling" the in-flight exception).
					e.pushScope(nil, false)
					e.locals.SetValue(ex, thrown)
					uw = e.evalStatement(catch.Body)
					e.popScope(nil)
					break
				}
			}
		}
	}

	// No matter the rt.Unwind instructions, be sure to invoke the finally part.
	if node.FinallyClause != nil {
		uwf := e.evalStatement(node.FinallyClause)

		// Any rt.Unwind information from the finally block overrides the try rt.Unwind that was in flight.
		if uwf != nil {
			uw = uwf
		}
	}

	return uw
}

func (e *evaluator) evalBreakStatement(node *ast.BreakStatement) *rt.Unwind {
	var label *tokens.Name
	if node.Label != nil {
		label = &node.Label.Ident
	}
	return rt.NewBreakUnwind(label)
}

func (e *evaluator) evalContinueStatement(node *ast.ContinueStatement) *rt.Unwind {
	var label *tokens.Name
	if node.Label != nil {
		label = &node.Label.Ident
	}
	return rt.NewContinueUnwind(label)
}

func (e *evaluator) evalIfStatement(node *ast.IfStatement) *rt.Unwind {
	// Evaluate the branches explicitly based on the result of the condition node.
	cond, uw := e.requireExpressionValue(node.Condition)
	if uw != nil {
		return uw
	}
	if cond.BoolValue() {
		return e.evalStatement(node.Consequent)
	} else if node.Alternate != nil {
		return e.evalStatement(*node.Alternate)
	}
	return nil
}

func (e *evaluator) evalSwitchStatement(node *ast.SwitchStatement) *rt.Unwind {
	// First evaluate the expression we are switching on.
	expr, uw := e.evalExpression(node.Expression)
	if uw != nil {
		return uw
	}

	// Next try to find a match; do this by walking all cases, in order, and checking for strict equality.
	fallen := false
	for _, caseNode := range node.Cases {
		match := false
		if fallen {
			// A fallthrough automatically executes the body without evaluating the clause.
			match = true
		} else if caseNode.Clause == nil {
			// A default style clause always matches.
			match = true
		} else {
			// Otherwise, evaluate the expression, and check for equality.
			clause, uw2 := e.evalExpression(*caseNode.Clause)
			if uw2 != nil {
				return uw2
			}
			match = e.evalBinaryOperatorEquals(expr, clause)
		}

		// If we got a match, execute the clause.
		if match {
			if uw = e.evalStatement(caseNode.Consequent); uw != nil {
				if uw.Break() && uw.Label() == nil {
					// A simple break from this case.
					break
				} else {
					// Anything else, get out of dodge.
					return uw
				}
			}

			// If we didn't encounter a break, we will fall through to the next case.
			fallen = true
		}
	}

	return nil
}

func (e *evaluator) evalLabeledStatement(node *ast.LabeledStatement) *rt.Unwind {
	// Evaluate the underlying statement; if it is breaking or continuing to this label, stop the rt.Unwind.
	uw := e.evalStatement(node.Statement)
	if uw != nil && uw.Label() != nil && *uw.Label() == node.Label.Ident {
		contract.Assert(uw.Continue() || uw.Break())
		// TODO[pulumi/lumi#214]: perform correct break/continue behavior when the label is affixed to a loop.
		uw = nil
	}
	return uw
}

func (e *evaluator) evalReturnStatement(node *ast.ReturnStatement) *rt.Unwind {
	var ret *rt.Object
	if node.Expression != nil {
		var uw *rt.Unwind
		if ret, uw = e.evalExpression(*node.Expression); uw != nil {
			// If the expression caused an rt.Unwind, propagate that and ignore the returned object.
			return uw
		}
	}
	return rt.NewReturnUnwind(ret)
}

func (e *evaluator) evalThrowStatement(node *ast.ThrowStatement) *rt.Unwind {
	thrown, uw := e.evalExpression(node.Expression)
	if uw != nil {
		// If the throw expression itself threw an exception, propagate that instead.
		return uw
	}
	contract.Assert(thrown != nil)
	return rt.NewThrowUnwind(thrown, node, e.stack)
}

func (e *evaluator) evalLoop(condition *ast.Expression, body ast.Statement, post *ast.Statement) *rt.Unwind {
	// So long as the condition evaluates to true, keep on visiting the body.
	for {
		var test *rt.Object
		if condition != nil {
			var uw *rt.Unwind
			if test, uw = e.requireExpressionValue(*condition); uw != nil {
				return uw
			}
		}
		if test == nil || test.BoolValue() {
			if uw := e.evalStatement(body); uw != nil {
				if uw.Continue() {
					contract.Assertf(uw.Label() == nil, "Labeled continue not yet supported")
					continue
				} else if uw.Break() {
					contract.Assertf(uw.Label() == nil, "Labeled break not yet supported")
					break
				} else {
					// If it's not a continue or break, return it right away.
					return uw
				}
			}
		} else {
			break
		}

		// Before looping around again, run the post statement, if there is one.
		if post != nil {
			if uw := e.evalStatement(*post); uw != nil {
				return uw
			}
		}
	}

	return nil
}

func (e *evaluator) evalWhileStatement(node *ast.WhileStatement) *rt.Unwind {
	// Just run the loop.
	return e.evalLoop(node.Condition, node.Body, nil)
}

func (e *evaluator) evalForStatement(node *ast.ForStatement) *rt.Unwind {
	// Now run the initialization code.
	if node.Init != nil {
		if uw := e.evalStatement(*node.Init); uw != nil {
			return uw
		}
	}

	// Now actually run the loop and post logic.
	return e.evalLoop(node.Condition, node.Body, node.Post)
}

func (e *evaluator) evalMultiStatement(node *ast.MultiStatement) *rt.Unwind {
	for _, stmt := range node.Statements {
		if uw := e.evalStatement(stmt); uw != nil {
			return uw
		}
	}
	return nil
}

func (e *evaluator) evalExpressionStatement(node *ast.ExpressionStatement) *rt.Unwind {
	// Just evaluate the expression, drop its object on the floor, and propagate its rt.Unwind information.
	_, uw := e.evalExpression(node.Expression)
	return uw
}

// Expressions

func (e *evaluator) evalExpression(node ast.Expression) (*rt.Object, *rt.Unwind) {
	contract.Assert(node != nil)
	if glog.V(7) {
		glog.V(7).Infof("Evaluating expression: %v", reflect.TypeOf(node))
	}

	// Simply switch on the node type and dispatch to the specific function, returning the object and rt.Unwind info.
	switch n := node.(type) {
	case *ast.NullLiteral:
		return e.evalNullLiteral(n)
	case *ast.BoolLiteral:
		return e.evalBoolLiteral(n)
	case *ast.NumberLiteral:
		return e.evalNumberLiteral(n)
	case *ast.StringLiteral:
		return e.evalStringLiteral(n)
	case *ast.ArrayLiteral:
		return e.evalArrayLiteral(n)
	case *ast.ObjectLiteral:
		return e.evalObjectLiteral(n)
	case *ast.LoadLocationExpression:
		return e.evalLoadLocationExpression(n)
	case *ast.LoadDynamicExpression:
		return e.evalLoadDynamicExpression(n)
	case *ast.TryLoadDynamicExpression:
		return e.evalTryLoadDynamicExpression(n)
	case *ast.NewExpression:
		return e.evalNewExpression(n)
	case *ast.InvokeFunctionExpression:
		return e.evalInvokeFunctionExpression(n)
	case *ast.LambdaExpression:
		return e.evalLambdaExpression(n)
	case *ast.UnaryOperatorExpression:
		return e.evalUnaryOperatorExpression(n)
	case *ast.BinaryOperatorExpression:
		return e.evalBinaryOperatorExpression(n)
	case *ast.CastExpression:
		return e.evalCastExpression(n)
	case *ast.IsInstExpression:
		return e.evalIsInstExpression(n)
	case *ast.TypeOfExpression:
		return e.evalTypeOfExpression(n)
	case *ast.ConditionalExpression:
		return e.evalConditionalExpression(n)
	case *ast.SequenceExpression:
		return e.evalSequenceExpression(n)
	default:
		contract.Failf("Unrecognized expression node kind: %v", node.GetKind())
		return nil, nil
	}
}

// requireExpressionValue evaluates an expression and ensures that it isn't latent; that is, it has a concrete value.
// If it is latent, the function returns a non-nil exception unwind object.
func (e *evaluator) requireExpressionValue(node ast.Expression) (*rt.Object, *rt.Unwind) {
	// TODO[pulumi/lumi#170]: eventually we should audit all uses of this routine and, for most if not all of them,
	//     make them work.  Doing so requires a fair bit of machinery around stepwise application of deployments.
	obj, uw := e.evalExpression(node)
	if uw != nil {
		return nil, uw
	} else if uw = e.rejectComputed(node, obj); uw != nil {
		return nil, uw
	}
	return obj, nil
}

// evalLValueExpression evaluates an expression for use as an l-value; in particular, this loads the target as a
// pointer/reference object, rather than as an ordinary value, so that it can be used in an assignment.  This is only
// valid on the subset of AST nodes that are legal l-values (very few of them, it turns out).
func (e *evaluator) evalLValueExpression(node ast.Expression) (*Location, *rt.Unwind) {
	switch n := node.(type) {
	case *ast.LoadLocationExpression:
		return e.evalLoadLocation(n, true)
	case *ast.LoadDynamicExpression:
		return e.evalLoadDynamic(n, true)
	case *ast.TryLoadDynamicExpression:
		return e.evalTryLoadDynamic(n, true)
	case *ast.UnaryOperatorExpression:
		contract.Assert(n.Operator == ast.OpDereference)
		obj, uw := e.evalUnaryOperatorExpressionFor(n, true)
		return &Location{e: e, Obj: obj}, uw
	default:
		contract.Failf("Unrecognized l-value expression type: %v", node.GetKind())
		return nil, nil
	}
}

func (e *evaluator) evalNullLiteral(node *ast.NullLiteral) (*rt.Object, *rt.Unwind) {
	return rt.Null, nil
}

func (e *evaluator) evalBoolLiteral(node *ast.BoolLiteral) (*rt.Object, *rt.Unwind) {
	return rt.Bools[node.Value], nil
}

func (e *evaluator) evalNumberLiteral(node *ast.NumberLiteral) (*rt.Object, *rt.Unwind) {
	return rt.NewNumberObject(node.Value), nil
}

func (e *evaluator) evalStringLiteral(node *ast.StringLiteral) (*rt.Object, *rt.Unwind) {
	return rt.NewStringObject(node.Value), nil
}

func (e *evaluator) evalArrayLiteral(node *ast.ArrayLiteral) (*rt.Object, *rt.Unwind) {
	// Fetch this expression type and assert that it's an array.
	ty := e.ctx.RequireType(node).(*symbols.ArrayType)

	// Now create the array data.
	var sz *int
	var arr []*rt.Pointer

	// If there's a node size, ensure it's a number, and initialize the array.
	if node.Size != nil {
		sze, uw := e.evalExpression(*node.Size)
		if uw != nil {
			return nil, uw
		} else if sze.Type().Computed() {
			// If the array size isn't known, then we will propagate a latent value in its place.
			return rt.NewComputedObject(ty, true, sze.ComputedValue().Sources), nil
		}
		sz := int(sze.NumberValue())
		if sz < 0 {
			// If the size is less than zero, raise a new error.
			return nil, e.NewNegativeArrayLengthException(*node.Size)
		}
		arr = make([]*rt.Pointer, sz)
	}

	// Allocate a new array object.
	arrobj := rt.NewArrayObject(ty.Element, &arr)

	// If there are elements, place them into the array.  This has two behaviors:
	//     1) if there is a size, there can be up to that number of elements, which are set;
	//     2) if there is no size, all of the elements are appended to the array.
	if node.Elements != nil {
		if sz == nil {
			// Right-size the array.
			arr = make([]*rt.Pointer, 0, len(*node.Elements))
		} else if len(*node.Elements) > *sz {
			// The element count exceeds the size; raise an error.
			return nil, e.NewIncorrectArrayElementCountException(node, *sz, len(*node.Elements))
		}

		for i, elem := range *node.Elements {
			elemobj, uw := e.evalExpression(elem)
			if uw != nil {
				return nil, uw
			}
			elemptr := rt.NewPointer(elemobj, false, nil, nil)
			if sz == nil {
				arr = append(arr, elemptr)
			} else {
				arr[i] = elemptr
			}
		}
	}

	return arrobj, nil
}

func (e *evaluator) evalObjectLiteral(node *ast.ObjectLiteral) (*rt.Object, *rt.Unwind) {
	ty := e.ctx.Types[node]

	// Allocate a new object of the right type, containing all of the properties pre-populated.
	obj := e.newObject(ty)

	if node.Properties != nil {
		// The binder already checked that the properties are legal, so we will simply store them as values.
		for _, init := range *node.Properties {
			// For dynamic types, we simply store the values in the bag of properties.  For all other types, we actually
			// require that the token be a class member token that references a valid property.  Note that we evaluate
			// the LHS before the RHS, so that evaluation order for computed properties is correct.
			var addr *rt.Pointer
			var property rt.PropertyKey
			switch p := init.(type) {
			case *ast.ObjectLiteralNamedProperty:
				id := p.Property.Tok
				if ty == types.Dynamic {
					property = rt.PropertyKey(id)
					addr = obj.GetPropertyAddr(property, true, true)
				} else {
					contract.Assert(id.HasClassMember())
					member := tokens.ClassMember(id).Name()
					property = rt.PropertyKey(member.Name())
					addr = obj.GetPropertyAddr(property, true, true)
				}
			case *ast.ObjectLiteralComputedProperty:
				name, uw := e.evalExpression(p.Property)
				if uw != nil {
					return nil, uw
				} else if name.Type().Computed() {
					// If we are setting a property on the object, and that property's name is dynamically computed
					// using a string whose value is not known, then we can't let the object be seen in a concrete
					// state.  Doing so would mean we can't assure latent propagation to operations on such properties.
					return rt.NewComputedObject(ty, true, name.ComputedValue().Sources), nil
				}
				property = rt.PropertyKey(name.StringValue())
				addr = obj.GetPropertyAddr(property, true, true)
			}

			// Evaluate and set the property.
			val, uw := e.evalExpression(init.Val())
			if uw != nil {
				return nil, uw
			}
			contract.Assert(val != nil)
			addr.Set(val)
		}
	}

	// Ensure we freeze anything that must be frozen.
	obj.FreezeReadonlyProperties()

	return obj, nil
}

// getObjectOrSuperProperty loads a property pointer from an object using the given property key.  It understands how
// to determine whether this is a `super` load, and bind it, and will adjust the resulting pointer accordingly.
func (e *evaluator) getObjectOrSuperProperty(
	obj *rt.Object, objexpr ast.Expression, k rt.PropertyKey, init bool, forWrite bool) (*rt.Pointer, *rt.Unwind) {
	// Ensure the object's class has been initialized.
	if obj != nil {
		if class, isclass := obj.Type().(*symbols.Class); isclass {
			if uw := e.ensureClassInit(class); uw != nil {
				return nil, uw
			}
		}
	}

	// If this member is being accessed using "super", we need to start our property search from the
	// superclass prototype, and not the object itself, so that we find the right value.
	var super *rt.Object
	if objexpr != nil {
		if ldloc, isldloc := objexpr.(*ast.LoadLocationExpression); isldloc {
			if ldloc.Name.Tok == tokens.Token(tokens.SuperVariable) {
				contract.Assert(ldloc.Object == nil)
				// The super expression's type will resolve to the parent class; use that for lookups.
				super = e.getPrototypeObject(e.ctx.RequireType(objexpr))
			}
		}
	}

	// If a superclass, use the right prototype.
	if super != nil {
		// Make sure to call GetPropertyAddrForThis to ensure the this parameter is adjusted to the object.
		return super.GetPropertyAddrForThis(obj, k, init, forWrite), nil
	}

	// Otherwise, simply fetch the property from the object directly.
	return obj.GetPropertyAddr(k, init, forWrite), nil
}

func (e *evaluator) evalLoadLocationExpression(node *ast.LoadLocationExpression) (*rt.Object, *rt.Unwind) {
	loc, uw := e.evalLoadLocation(node, false)
	if uw != nil {
		return nil, uw
	}
	return loc.Read(node)
}

func (e *evaluator) newLocation(node diag.Diagable, sym symbols.Symbol,
	pv *rt.Pointer, ty symbols.Type, this *rt.Object, lval bool) *Location {
	// If this is an l-value, return a pointer to the object; otherwise, return the raw object.
	var obj *rt.Object
	if lval {
		obj = rt.NewPointerObject(ty, pv)
	} else {
		obj = pv.Obj()
	}

	if glog.V(9) {
		glog.V(9).Infof("Loaded location of type '%v' from symbol '%v': lval=%v current=%v",
			ty.Token(), sym.Token(), lval, pv.Obj())
	}

	return &Location{
		e:      e,
		This:   this,
		Name:   sym.Name(),
		Lval:   lval,
		Obj:    obj,
		Getter: pv.Getter(),
		Setter: pv.Setter(),
	}
}

type Location struct {
	e      *evaluator       // the evaluator that produced this location.
	This   *rt.Object       // the target object, if any.
	Name   tokens.Name      // the simple name of the variable.
	Lval   bool             // whether the result is an lval.
	Obj    *rt.Object       // the resulting object (pointer if lval, object otherwise).
	Getter symbols.Function // the getter function, if any.
	Setter symbols.Function // the setter function, if any.
}

func (loc *Location) Get(node diag.Diagable) (*rt.Object, *rt.Unwind) {
	if loc.Getter != nil {
		// If there is a getter, invoke it.
		contract.Assert(loc.This != nil)
		return loc.e.evalCallSymbol(node, loc.Getter, loc.This)
	}

	// Otherwise, just return the object directly.
	return loc.Obj, nil
}

func (loc *Location) Set(node diag.Diagable, val *rt.Object) *rt.Unwind {
	if loc.Setter != nil {
		// If the location has a setter, use that for the assignment.
		contract.Assert(loc.This != nil)
		if _, uw := loc.e.evalCallSymbol(node, loc.Setter, loc.This, val); uw != nil {
			return uw
		}
	} else {
		// Otherwise, perform a straightforward assignment, invoking the variable assignment if necessary.
		ptr := loc.Obj.PointerValue()
		if ptr.Readonly() {
			// If the pointer is readonly, however, we will disallow the assignment.
			loc.e.Diag().Errorf(errors.ErrorIllegalReadonlyLValue.At(node))
		} else {
			ptr.Set(val)
		}
	}
	return nil
}

func (loc *Location) Read(node diag.Diagable) (*rt.Object, *rt.Unwind) {
	if loc.Getter != nil {
		// If the location has a getter, use that for the assignment.
		contract.Assert(loc.This != nil)
		return loc.e.evalCallSymbol(node, loc.Getter, loc.This)
	}
	// Otherwise, just return the object directly.
	return loc.Obj, nil
}

// evalLoadLocation evaluates and loads information about the target.  It takes an lval bool which
// determines whether the resulting location object is an lval (pointer) or regular object.
func (e *evaluator) evalLoadLocation(node *ast.LoadLocationExpression, lval bool) (*Location, *rt.Unwind) {
	// If there's a target object, evaluate it.
	var this *rt.Object
	var thisexpr ast.Expression
	if node.Object != nil {
		thisexpr = *node.Object
		var uw *rt.Unwind
		if this, uw = e.evalExpression(thisexpr); uw != nil {
			return nil, uw
		}
	}

	// Create a pointer to the target location.
	var pv *rt.Pointer
	var ty symbols.Type
	var sym symbols.Symbol
	tok := node.Name.Tok
	if tok.Simple() {
		// If there is no object and the name is simple, it refers to a local variable in the current scope.
		// For more "sophisticated" lookups, in the hierarchy of scopes, a load dynamic operation must be utilized.
		contract.Assert(this == nil)
		loc := e.locals.Lookup(tok.Name())
		contract.Assert(loc != nil)
		pv = e.locals.GetValueAddr(loc, true)
		ty = loc.Type()
		sym = loc
	} else {
		sym = e.ctx.LookupSymbol(node.Name, tok, false)
		var uw *rt.Unwind
		pv, ty, uw = e.evalLoadSymbolLocation(node, sym, this, thisexpr, lval)
		if uw != nil {
			return nil, uw
		}
	}

	return e.newLocation(node, sym, pv, ty, this, lval), nil
}

// chaseExports walks the referent chain of an export until it hits a real symbol, and then returns that.
func (e *evaluator) chaseExports(sym symbols.Symbol) symbols.Symbol {
	for {
		export, isexport := sym.(*symbols.Export)
		if !isexport {
			break
		}
		// Simply chase the referent symbol until we bottom out on something useful.
		contract.Assertf(export.Referent != sym, "Unexpected self-referential export token")
		sym = export.Referent
		contract.Assertf(sym != nil, "Expected export '%v' to resolve to a token", export.Node.Referent.Tok)
	}
	return sym
}

func (e *evaluator) evalLoadSymbolLocation(node diag.Diagable, sym symbols.Symbol,
	this *rt.Object, thisexpr ast.Expression, lval bool) (*rt.Pointer, symbols.Type, *rt.Unwind) {
	contract.Assert(sym != nil) // don't issue errors; we shouldn't ever get here if verification failed.
	sym = e.chaseExports(sym)   // chase the export down until we get a real symbol.

	// Look up the symbol property in the right place.  Note that because this is a static load, we intentionally
	// do not perform any lazily initialization of missing property slots; they must exist.  But we still need to
	// load from the object in case one of the properties was overwritten.  The sole exception is for `dynamic`.
	var pv *rt.Pointer
	var ty symbols.Type
	switch s := sym.(type) {
	case symbols.ClassMember:
		// Consult either the statics map or the object's property based on the kind of symbol.  Note that we do
		// this even for class functions so that in case they are replaced or overridden in derived types, we get
		// the expected "virtual" dispatch behavior.  The one special case is constructors, where we intentionally
		// return a statically resolved symbol (since they aren't stored as properties and to support `super`).
		k := rt.PropertyKey(sym.Name())
		if s.Static() {
			contract.Assert(this == nil)
			// Ensure the class is initialized.
			class := s.MemberParent()
			if uw := e.ensureClassInit(class); uw != nil {
				return nil, nil, uw
			}

			// Now fetch the statics and lookup the property.
			statics := e.getClassStatics(class)
			pv = statics.GetAddr(k)
			contract.Assertf(pv != nil, "Missing static class member '%v'", s.Token())
			contract.Assertf(pv.Getter() == nil, "Static class property getters unexpected (%v)", s)
			contract.Assertf(pv.Setter() == nil, "Static class property setters unexpected (%v)", s)
		} else {
			contract.Assert(this != nil)
			if uw := e.checkThis(node, this); uw != nil {
				return nil, nil, uw
			}
			dynload := this.Type() == types.Dynamic
			var uw *rt.Unwind
			if pv, uw = e.getObjectOrSuperProperty(this, thisexpr, k, dynload, lval); uw != nil {
				return nil, nil, uw
			}
			contract.Assertf(pv != nil, "Missing instance class member '%v'", s.Token())
		}
		ty = s.Type()
		contract.Assert(ty != nil)
	case symbols.ModuleMemberProperty:
		module := s.MemberParent()

		// Ensure this module has been initialized.
		if uw := e.ensureModuleInit(module); uw != nil {
			return nil, nil, uw
		}

		// Search the globals table for this module's members.
		contract.Assert(this == nil)
		k := rt.PropertyKey(s.Name())
		globals := e.getModuleGlobals(module)
		pv = globals.Properties().GetAddr(k)
		contract.Assertf(pv != nil, "Missing module member '%v'", s.Token())
		contract.Assertf(pv.Getter() == nil, "Module property getters unexpected (%v)", s)
		contract.Assertf(pv.Setter() == nil, "Module property setters unexpected (%v)", s)
		ty = s.MemberType()
		contract.Assert(ty != nil)
	default:
		contract.Failf("Unexpected symbol token '%v' kind during load expression: %v",
			sym.Token(), reflect.TypeOf(sym))
	}

	return pv, ty, nil
}

func (e *evaluator) LoadLocation(tree diag.Diagable, sym symbols.Symbol, this *rt.Object, lval bool) *Location {
	pv, ty, uw := e.evalLoadSymbolLocation(tree, sym, this, nil, lval)
	contract.Assertf(uw == nil, "Unexpected unwind; possible nil 'this' for instance method")
	return e.newLocation(tree, sym, pv, ty, this, lval)
}

// checkThis checks a this object, raising a runtime error if it is the runtime null value.
func (e *evaluator) checkThis(node diag.Diagable, this *rt.Object) *rt.Unwind {
	contract.Assert(this != nil) // binder should catch cases where this isn't true
	if this.Type() == types.Null {
		return e.NewNullObjectException(node)
	}
	return nil
}

func (e *evaluator) evalLoadDynamicExpression(node *ast.LoadDynamicExpression) (*rt.Object, *rt.Unwind) {
	loc, uw := e.evalLoadDynamic(node, false)
	if uw != nil {
		return nil, uw
	}
	return loc.Read(node)
}

func (e *evaluator) evalLoadDynamic(node *ast.LoadDynamicExpression, lval bool) (*Location, *rt.Unwind) {
	return e.evalLoadDynamicCore(node, node.Object, node.Name, false, lval)
}

func (e *evaluator) evalTryLoadDynamicExpression(node *ast.TryLoadDynamicExpression) (*rt.Object, *rt.Unwind) {
	loc, uw := e.evalTryLoadDynamic(node, false)
	if uw != nil {
		return nil, uw
	}
	return loc.Read(node)
}

func (e *evaluator) evalTryLoadDynamic(node *ast.TryLoadDynamicExpression, lval bool) (*Location, *rt.Unwind) {
	return e.evalLoadDynamicCore(node, node.Object, node.Name, true, lval)
}

func (e *evaluator) evalLoadDynamicCore(node ast.Node, objexpr *ast.Expression, nameexpr ast.Expression,
	try bool, lval bool) (*Location, *rt.Unwind) {
	// Evaluate the object and then the property expression.
	var uw *rt.Unwind
	var this *rt.Object
	var thisexpr ast.Expression
	if objexpr != nil {
		thisexpr = *objexpr
		if this, uw = e.evalExpression(thisexpr); uw != nil {
			return nil, uw
		}

		// Check that the object isn't null; if it is, raise an exception.
		if uw = e.checkThis(node, this); uw != nil {
			return nil, uw
		}

	}
	var name *rt.Object
	if name, uw = e.evalExpression(nameexpr); uw != nil {
		return nil, uw
	}

	var pv *rt.Pointer
	var key tokens.Name
	if (this != nil && this.Type().Computed()) || name.Type().Computed() {
		// If the object or name are latent, we can't possibly proceed, because we do not know what to lookup.
		var comps []*rt.Object
		if this.Type().Computed() {
			comps = append(comps, this.ComputedValue().Sources...)
		}
		if name.Type().Computed() {
			comps = append(comps, name.ComputedValue().Sources...)
		}
		lat := rt.NewComputedObject(types.Dynamic, true, comps)
		pv = rt.NewPointer(lat, false, nil, nil)
	} else {
		// Otherwise, go ahead and search the object for a property with the given name.
		if name.Type() == types.Number {
			_, isarr := this.Type().(*symbols.ArrayType)
			contract.Assertf(isarr, "Expected an array for numeric dynamic load index")
			ix := int(name.NumberValue())
			arrv := this.ArrayValue()
			// TODO[pulumi/lumi#70]: Although storing arrays as arrays is fine for many circumstances, there are two
			//     situations that could cause us troubles with ECMAScript compliance.  First, negative indices are fine in
			//     ECMAScript.  Second, sparse arrays can be represented more efficiently as a "bag of properties" than as a
			//     true array that needs to be resized (possibly growing to become enormous in memory usage).
			// TODO[pulumi/lumi#70]: We are emulating "ECMAScript-like" array accesses, where -- just like ordinary
			//     property accesses below -- we will permit indexes that we've never seen before.  Out of bounds should
			//     yield `undefined`, rather than the usual case of throwing an exception, for example.  And such
			//     assignments are to be permitted.  This will cause troubles down the road when we do other languages that
			//     reject out of bounds accesses e.g. Python.  An alternative approach would be to require ECMAScript to
			//     use a runtime library anytime an array is accessed, translating exceptions like this into `undefined`s.
			if ix >= len(*arrv) && (lval || try) {
				newarr := make([]*rt.Pointer, ix+1)
				copy(*arrv, newarr)
				*arrv = newarr
			}
			if ix < len(*arrv) {
				pv = (*arrv)[ix]
				if pv == nil && (lval || try) {
					nul := rt.Null
					pv = rt.NewPointer(nul, false, nil, nil)
					(*arrv)[ix] = pv
				}
			}
		} else {
			contract.Assertf(name.Type() == types.String, "Expected dynamic load name to be a string")
			key = tokens.Name(name.StringValue())
			if thisexpr == nil {
				pv = e.getDynamicNameAddr(key, lval)
			} else {
				var uw *rt.Unwind
				if pv, uw = e.getObjectOrSuperProperty(
					this, thisexpr, rt.PropertyKey(key), lval || try, lval); uw != nil {
					return nil, uw
				}
			}
		}
	}

	if pv == nil && try {
		// If this is a try load and we couldn't find the name,
		pv = rt.NewPointer(rt.Null, false, nil, nil)
	}

	if pv == nil {
		// If the result is nil, then the name is not defined.  In the event of a try load, we will have substituted a
		// null already above, so if we got here, we need to propagate an exception.
		contract.Assert(!try)
		contract.Assert(!lval)
		return nil, e.NewNameNotDefinedException(node, key)
	}

	// If this isn't for an l-value, return the raw object.  Otherwise, make sure it's not readonly, and return it.
	var obj *rt.Object
	if lval {
		obj = rt.NewPointerObject(types.Dynamic, pv)
	} else {
		obj = pv.Obj()
	}
	contract.Assert(obj != nil)

	return &Location{
		e:      e,
		This:   this,
		Name:   key,
		Lval:   lval,
		Obj:    obj,
		Getter: pv.Getter(),
		Setter: pv.Setter(),
	}, nil
}

func getDynamicNameAddrCore(locals rt.Environment, globals *rt.Object, key tokens.Name) *rt.Pointer {
	if loc := locals.Lookup(key); loc != nil {
		return locals.GetValueAddr(loc, true) // create a slot, we know the declaration exists.
	}
	// If it didn't exist in the lexical scope, check the module's globals.
	pkey := rt.PropertyKey(key)
	return globals.Properties().GetAddr(pkey) // look for a global by this name, but don't allocate one.
}

func (e *evaluator) getDynamicNameAddr(key tokens.Name, lval bool) *rt.Pointer {
	globals := e.getModuleGlobals(e.ctx.Currmodule)
	pv := getDynamicNameAddrCore(e.locals, globals, key)

	// If not found and this is the target of a load, allocate a slot.
	if pv == nil && lval {
		if e.fnc != nil && e.fnc.SpecialModInit() && e.locals.Activation() {
			pkey := rt.PropertyKey(key)
			pv = globals.Properties().GetInitAddr(pkey)
		} else {
			loc := symbols.NewSpecialVariableSym(key, types.Dynamic)
			e.locals.MustRegister(loc)
			pv = e.locals.GetValueAddr(loc, true)
		}
	}

	return pv
}

func (e *evaluator) evalNewExpression(node *ast.NewExpression) (*rt.Object, *rt.Unwind) {
	// Fetch the type of this expression; that's the kind of object we are allocating.
	ty := e.ctx.RequireType(node)
	// Now actually perform the new operation.
	return e.evalNew(node, ty, node.Arguments)
}

// evalNew performs a new operation on the given type with the given arguments.
func (e *evaluator) evalNew(node diag.Diagable, t symbols.Type, args *[]*ast.CallArgument) (*rt.Object, *rt.Unwind) {
	// TODO[pulumi/lumi#176]: if a dynamic invoke, we want a runtime exceptions, not assertions, for failures below.

	// Create a object of the right type, containing all of the properties pre-populated.
	obj := e.newObject(t)

	// Evaluate the arguments in order.
	var argobjs []*rt.Object
	if args != nil {
		for _, arg := range *args {
			argobj, uw := e.evalExpression(arg.Expr)
			if uw != nil {
				return nil, uw
			}
			argobjs = append(argobjs, argobj)
		}
	}

	// See if there is a constructor or if this is a record.  If not, just return a fresh object.
	if ctor := t.Ctor(); ctor != nil {
		contract.Assertf(ctor.Signature().Return == nil || ctor.Signature().Return == types.Dynamic,
			"Expected ctor %v to have a nil (or dynamic) return; got %v", ctor, ctor.Signature().Return)
		// Now dispatch the function call using the fresh object as the constructor's `this` argument.
		if _, uw := e.evalCallSymbol(node, ctor, obj, argobjs...); uw != nil {
			return nil, uw
		}
	} else if args != nil && t.Record() {
		// If this is a record type, we can still initialize it much like an object literal, using named properties.
		// This only works provided the arguments are named and each one maps to a primary property on the type.
		for i, arg := range *args {
			contract.Assertf(arg.Name != nil, "Expected only named args for new of a record type")
			id := tokens.ClassMemberName(arg.Name.Ident)
			contract.Assertf(t.TypeMembers()[id] != nil, "Expected named arg %v to match a type member", id)
			contract.Assertf(t.TypeMembers()[id].Primary(), "Expected named arg %v to match a primary member", id)
			val := argobjs[i]
			prop := rt.PropertyKey(id)
			addr := obj.GetPropertyAddr(prop, true, true)
			addr.Set(val)
		}
	} else {
		contract.Assertf(args == nil || len(*args) == 0,
			"No constructor found for non-record type %v, yet the new expression had %v args", t, len(*args))
	}

	// If there are post-construction hooks, run them right now.  Note that we intentionally invoke this after
	// construction but before object freezing, in case the hook wants to mutate readonly properties.  In a sense, this
	// hook becomes an extension of the constructor itself.
	if e.hooks != nil {
		e.hooks.OnObjectInit(node, obj)
	}

	// Finally, ensure that all readonly properties are frozen now.
	obj.FreezeReadonlyProperties()

	return obj, nil
}

func (e *evaluator) evalInvokeFunctionExpression(node *ast.InvokeFunctionExpression) (*rt.Object, *rt.Unwind) {
	// Evaluate the function that we are meant to invoke.  Note that at the moment we reject latent types; we could
	// simply propagate a latent value with the expected return type, however that would risk covering up code paths
	// that contain conditionals, something that we don't permit until pulumi/lumi#170 is handled.
	fncobj, uw := e.requireExpressionValue(node.Function)
	if uw != nil {
		return nil, uw
	}

	// See if this is a dynamic invocation -- these are validated differently.
	dynamic := (e.ctx.RequireType(node.Function) == types.Dynamic)

	// Ensure that this actually led to a function; this is guaranteed by the binder.
	var fnc rt.FuncStub
	switch t := fncobj.Type().(type) {
	case *symbols.FunctionType:
		fnc = fncobj.FunctionValue()
	case *symbols.PrototypeType:
		contract.Assertf(dynamic, "Prototype invocation is only valid for dynamic invokes")
		// For dynamic invokes, we permit invocation of class prototypes (a "new").
		ot := t.Type // this is the type of object to create.
		return e.evalNew(node, ot, node.Arguments)
	default:
		// If dynamic, raise an exception; otherwise, assert, since the IL was malformed and should have been caught
		// during binding/verification time.
		if dynamic {
			return nil, e.NewIllegalInvokeTargetException(node.Function, t)
		}
		contract.Failf("Expected function expression to yield a function type")
	}

	// Now evaluate the arguments to the function, in order.
	var args []*rt.Object
	if node.Arguments != nil {
		for _, arg := range *node.Arguments {
			argobj, uw := e.evalExpression(arg.Expr)
			if uw != nil {
				return nil, uw
			}
			args = append(args, argobj)
		}
	}

	// Finally, actually dispatch the call; this will create the activation frame, etc. for us.
	return e.evalCallFuncStub(node, fnc, args...)
}

func (e *evaluator) evalLambdaExpression(node *ast.LambdaExpression) (*rt.Object, *rt.Unwind) {
	// To create a lambda object we will simply produce a function object that can invoke it.  Lambdas also uniquely
	// capture the current environment, including the this variable.
	sig := e.ctx.RequireType(node).(*symbols.FunctionType)
	moduleObject := e.getModuleGlobals(e.ctx.Currmodule)
	obj := rt.NewFunctionObjectFromLambda(node, sig, e.locals, moduleObject)
	return obj, nil
}

func (e *evaluator) evalUnaryOperatorExpression(node *ast.UnaryOperatorExpression) (*rt.Object, *rt.Unwind) {
	return e.evalUnaryOperatorExpressionFor(node, false)
}

func (e *evaluator) evalUnaryOperatorExpressionFor(node *ast.UnaryOperatorExpression, lval bool) (*rt.Object, *rt.Unwind) {
	contract.Assertf(!lval || node.Operator == ast.OpDereference, "Only dereference unary ops support l-values")

	// Evaluate the operand and prepare to use it.
	var opand *rt.Object
	var opandloc *Location
	op := node.Operator
	if op == ast.OpAddressof || op == ast.OpPlusPlus || op == ast.OpMinusMinus {
		// These operators require an l-value; so we bind the expression a bit differently.
		loc, uw := e.evalLValueExpression(node.Operand)
		if uw != nil {
			return nil, uw
		}
		opand, uw = loc.Read(node)
		if uw != nil {
			return nil, uw
		}
		opandloc = loc
	} else {
		// Otherwise, we just need to evaluate the operand as usual.
		var uw *rt.Unwind
		if opand, uw = e.evalExpression(node.Operand); uw != nil {
			return nil, uw
		}
	}

	// See if the operand is a computed type.  If yes, treat it differently.
	if opand.Type().Computed() {
		comps := opand.ComputedValue().Sources
		switch op {
		case ast.OpDereference:
			// The target is a pointer; return the underlying pointer element.
			pt := opand.Type().(*symbols.ComputedType).Element
			et := pt.(*symbols.PointerType).Element
			return rt.NewComputedObject(et, true, comps), nil
		case ast.OpAddressof:
			// The target is a pointer; return the actual pointer.
			pt := opand.Type().(*symbols.ComputedType).Element
			return rt.NewComputedObject(pt, true, comps), nil
		case ast.OpLogicalNot:
			// The target is a boolean; propagate a latent boolean.
			return rt.NewComputedObject(types.Bool, true, comps), nil
		case ast.OpUnaryPlus, ast.OpUnaryMinus, ast.OpBitwiseNot,
			ast.OpPlusPlus, ast.OpMinusMinus:
			// All these operators deal with numbers; so, propagate a latent number.
			return rt.NewComputedObject(types.Number, true, comps), nil
		default:
			contract.Failf("Unrecognized unary operator: %v", op)
			return nil, nil
		}
	}

	// The value is known; switch on the operator and perform its specific operation.
	switch op {
	case ast.OpDereference:
		// The target is a pointer.  If this is for an l-value, just return it as-is; otherwise, dereference it.
		ptr := opand.PointerValue()
		contract.Assert(ptr != nil)
		if lval {
			return opand, nil
		}
		return ptr.Obj(), nil
	case ast.OpAddressof:
		// The target is an l-value, load its address.
		contract.Assert(opand.PointerValue() != nil)
		return opand, nil
	case ast.OpUnaryPlus:
		// The target is a number; simply fetch it (asserting its value), and + it.
		return rt.NewNumberObject(+opand.NumberValue()), nil
	case ast.OpUnaryMinus:
		// The target is a number; simply fetch it (asserting its value), and - it.
		return rt.NewNumberObject(-opand.NumberValue()), nil
	case ast.OpLogicalNot:
		// The target is a boolean; simply fetch it (asserting its value), and ! it.
		return rt.Bools[!opand.BoolValue()], nil
	case ast.OpBitwiseNot:
		// The target is a number; simply fetch it (asserting its value), and ^ it (similar to C's ~ operator).
		return rt.NewNumberObject(float64(^int64(opand.NumberValue()))), nil
	case ast.OpPlusPlus:
		// The target is an l-value; we must load it, ++ it, and return the appropriate prefix/postfix value.
		ptr := opand.PointerValue()
		old := ptr.Obj()
		val := old.NumberValue()
		new := rt.NewNumberObject(val + 1)
		if uw := opandloc.Set(node.Operand, new); uw != nil {
			return nil, uw
		}
		if node.Postfix {
			return old, nil
		}
		return new, nil
	case ast.OpMinusMinus:
		// The target is an l-value; we must load it, -- it, and return the appropriate prefix/postfix value.
		ptr := opand.PointerValue()
		old := ptr.Obj()
		val := old.NumberValue()
		new := rt.NewNumberObject(val - 1)
		if uw := opandloc.Set(node.Operand, new); uw != nil {
			return nil, uw
		}
		if node.Postfix {
			return old, nil
		}
		return new, nil
	default:
		contract.Failf("Unrecognized unary operator: %v", op)
		return nil, nil
	}
}

func (e *evaluator) evalBinaryOperatorExpression(node *ast.BinaryOperatorExpression) (*rt.Object, *rt.Unwind) {
	// Evaluate the operands and prepare to use them.  First left, then right.
	var lhs *rt.Object
	var lhsloc *Location
	op := node.Operator
	if ast.IsAssignmentBinaryOperator(op) {
		var uw *rt.Unwind
		if lhsloc, uw = e.evalLValueExpression(node.Left); uw != nil {
			return nil, uw
		}
		lhs, uw = lhsloc.Read(node)
		if uw != nil {
			return nil, uw
		}
	} else {
		var uw *rt.Unwind
		if lhs, uw = e.evalExpression(node.Left); uw != nil {
			return nil, uw
		}
	}

	// For the logical && and ||, we will only evaluate the rhs it if the lhs was true.
	if ast.IsConditionalBinaryOperator(op) {
		if uw := e.rejectComputed(node.Left, lhs); uw != nil {
			return nil, uw
		}
		if lhs.BoolValue() {
			return e.evalExpression(node.Right)
		}
		return rt.False, nil
	}

	// Otherwise, just evaluate the rhs and prepare to evaluate the operator.
	rhs, uw := e.evalExpression(node.Right)
	if uw != nil {
		return nil, uw
	}

	// Accumulate all the computed sources, if any, so that we may propagate them.
	var comps []*rt.Object
	if lhs.Type().Computed() {
		comps = append(comps, lhs.ComputedValue().Sources...)
	}
	if rhs.Type().Computed() {
		comps = append(comps, rhs.ComputedValue().Sources...)
	}

	// If the operator involves computed operations, return a computed of the right type.
	if len(comps) > 0 {
		if ast.IsArithmeticBinaryOperator(op) || ast.IsBitwiseBinaryOperator(op) {
			if op == ast.OpAdd && types.CanConvert(rhs.Type(), types.String) {
				// + can involve strings for concatenation.
				return rt.NewComputedObject(types.String, true, comps), nil
			}
			// All other arithmetic operators deal in terms of numbers.
			return rt.NewComputedObject(types.Number, true, comps), nil
		} else if ast.IsAssignmentBinaryOperator(op) {
			if op == ast.OpAssign {
				// = is an arbitrary type, just use the rhs.
				return rt.NewComputedObject(rhs.Type().(*symbols.ComputedType).Element, true, comps), nil
			} else if op == ast.OpAssignSum && types.CanConvert(rhs.Type(), types.String) {
				// += can involve strings for concatenation.
				return rt.NewComputedObject(types.String, true, comps), nil
			}
			// All other assignment operators deal in terms of numbers.
			return rt.NewComputedObject(types.Number, true, comps), nil
		} else if ast.IsRelationalBinaryOperator(op) {
			return rt.NewComputedObject(types.Bool, true, comps), nil
		}
		contract.Failf("Expected to resolve latent binary operator expression for operator: %v", op)
	}

	// Switch on operator to perform the operator's effects.
	// TODO[pulumi/lumi#176]: anywhere there is type coercion to/from float64/int64/etc., we should be skeptical.
	//     Because our numeric type system is float64-based -- i.e., "JSON-like" -- we often find ourselves doing
	//     operations on floats that honestly don't make sense (like shifting, masking, and whatnot).  If there is a
	//     type coercion, Golang (rightfully) doesn't support an operator on numbers of that type.  I suspect we will
	//     eventually want to consider integer types in LumiIL, and/or verify that numbers aren't outside of the legal
	//     range as part of verification, and then push the responsibility for presenting valid LumiIL with any required
	//     conversions bacp up to the compilers (compile-time, runtime, or othwerwise, per the language semantics).
	switch op {
	// Arithmetic operators
	case ast.OpAdd:
		// If the lhs/rhs are strings, concatenate them; if numbers, + them.
		if types.CanConvert(rhs.Type(), types.String) {
			return rt.NewStringObject(lhs.StringValue() + rhs.StringValue()), nil
		}
		return rt.NewNumberObject(lhs.NumberValue() + rhs.NumberValue()), nil
	case ast.OpSubtract:
		// Both targets are numbers; fetch them (asserting their types), and - them.
		return rt.NewNumberObject(lhs.NumberValue() - rhs.NumberValue()), nil
	case ast.OpMultiply:
		// Both targets are numbers; fetch them (asserting their types), and * them.
		return rt.NewNumberObject(lhs.NumberValue() * rhs.NumberValue()), nil
	case ast.OpDivide:
		// Both targets are numbers; fetch them (asserting their types), and / them.
		return rt.NewNumberObject(lhs.NumberValue() / rhs.NumberValue()), nil
	case ast.OpRemainder:
		// Both targets are numbers; fetch them (asserting their types), and % them.
		return rt.NewNumberObject(float64(int64(lhs.NumberValue()) % int64(rhs.NumberValue()))), nil
	case ast.OpExponentiate:
		// Both targets are numbers; fetch them (asserting their types), and raise lhs to rhs's power.
		return rt.NewNumberObject(math.Pow(lhs.NumberValue(), rhs.NumberValue())), nil

	// Bitwise operators
	// TODO[pulumi/lumi#176]: the ECMAScript specification for bitwise operators is a fair bit more complicated than
	//     these; for instance, shifts mask out all but the least significant 5 bits of the rhs.  If we don't do it
	//     here, LumiJS should; e.g. see https://www.ecma-international.org/ecma-262/7.0/#sec-left-shift-operator.
	case ast.OpBitwiseShiftLeft:
		// Both targets are numbers; fetch them (asserting their types), and << them.
		// TODO[pulumi/lumi#176]: issue an error if rhs is negative.
		return rt.NewNumberObject(float64(int64(lhs.NumberValue()) << uint(rhs.NumberValue()))), nil
	case ast.OpBitwiseShiftRight:
		// Both targets are numbers; fetch them (asserting their types), and >> them.
		// TODO[pulumi/lumi#176]: issue an error if rhs is negative.
		return rt.NewNumberObject(float64(int64(lhs.NumberValue()) >> uint(rhs.NumberValue()))), nil
	case ast.OpBitwiseAnd:
		// Both targets are numbers; fetch them (asserting their types), and & them.
		return rt.NewNumberObject(float64(int64(lhs.NumberValue()) & int64(rhs.NumberValue()))), nil
	case ast.OpBitwiseOr:
		// Both targets are numbers; fetch them (asserting their types), and | them.
		return rt.NewNumberObject(float64(int64(lhs.NumberValue()) | int64(rhs.NumberValue()))), nil
	case ast.OpBitwiseXor:
		// Both targets are numbers; fetch them (asserting their types), and ^ them.
		return rt.NewNumberObject(float64(int64(lhs.NumberValue()) ^ int64(rhs.NumberValue()))), nil

	// Assignment operators
	case ast.OpAssign:
		// The target is an l-value; just overwrite its value, and yield the new value as the result.
		if uw := lhsloc.Set(node.Left, rhs); uw != nil {
			return nil, uw
		}
		return rhs, nil
	case ast.OpAssignSum:
		var val *rt.Object
		ptr := lhs.PointerValue()
		if ptr.Obj().Type() == types.String {
			// If the lhs/rhs are strings, just concatenate += and yield the new value as a result.
			val = rt.NewStringObject(ptr.Obj().StringValue() + rhs.StringValue())
		} else {
			// Otherwise, the target is a numeric l-value; just += to it, and yield the new value as the result.
			val = rt.NewNumberObject(ptr.Obj().NumberValue() + rhs.NumberValue())
		}
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil
	case ast.OpAssignDifference:
		// The target is a numeric l-value; just -= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := rt.NewNumberObject(ptr.Obj().NumberValue() - rhs.NumberValue())
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil
	case ast.OpAssignProduct:
		// The target is a numeric l-value; just *= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := rt.NewNumberObject(ptr.Obj().NumberValue() * rhs.NumberValue())
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil
	case ast.OpAssignQuotient:
		// The target is a numeric l-value; just /= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := rt.NewNumberObject(ptr.Obj().NumberValue() / rhs.NumberValue())
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil
	case ast.OpAssignRemainder:
		// The target is a numeric l-value; just %= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := rt.NewNumberObject(float64(int64(ptr.Obj().NumberValue()) % int64(rhs.NumberValue())))
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil
	case ast.OpAssignExponentiation:
		// The target is a numeric l-value; just raise to rhs as a power, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := rt.NewNumberObject(math.Pow(ptr.Obj().NumberValue(), rhs.NumberValue()))
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil
	case ast.OpAssignBitwiseShiftLeft:
		// The target is a numeric l-value; just <<= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := rt.NewNumberObject(float64(int64(ptr.Obj().NumberValue()) << uint(rhs.NumberValue())))
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil
	case ast.OpAssignBitwiseShiftRight:
		// The target is a numeric l-value; just >>= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := rt.NewNumberObject(float64(int64(ptr.Obj().NumberValue()) >> uint(rhs.NumberValue())))
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil
	case ast.OpAssignBitwiseAnd:
		// The target is a numeric l-value; just &= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := rt.NewNumberObject(float64(int64(ptr.Obj().NumberValue()) & int64(rhs.NumberValue())))
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil
	case ast.OpAssignBitwiseOr:
		// The target is a numeric l-value; just |= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := rt.NewNumberObject(float64(int64(ptr.Obj().NumberValue()) | int64(rhs.NumberValue())))
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil
	case ast.OpAssignBitwiseXor:
		// The target is a numeric l-value; just ^= rhs to it, and yield the new value as the result.
		ptr := lhs.PointerValue()
		val := rt.NewNumberObject(float64(int64(ptr.Obj().NumberValue()) ^ int64(rhs.NumberValue())))
		if uw := lhsloc.Set(node.Left, val); uw != nil {
			return nil, uw
		}
		return val, nil

	// Relational operators
	case ast.OpLt:
		// The targets are numbers; just compare them with < and yield the boolean result.
		return rt.Bools[lhs.NumberValue() < rhs.NumberValue()], nil
	case ast.OpLtEquals:
		// The targets are numbers; just compare them with <= and yield the boolean result.
		return rt.Bools[lhs.NumberValue() <= rhs.NumberValue()], nil
	case ast.OpGt:
		// The targets are numbers; just compare them with > and yield the boolean result.
		return rt.Bools[lhs.NumberValue() > rhs.NumberValue()], nil
	case ast.OpGtEquals:
		// The targets are numbers; just compare them with >= and yield the boolean result.
		return rt.Bools[lhs.NumberValue() >= rhs.NumberValue()], nil
	case ast.OpEquals:
		// Equality checking handles many object types, so defer to a helper for it.
		return rt.Bools[e.evalBinaryOperatorEquals(lhs, rhs)], nil
	case ast.OpNotEquals:
		// Just return the inverse of what the operator equals function itself returns.
		return rt.Bools[!e.evalBinaryOperatorEquals(lhs, rhs)], nil

	default:
		contract.Failf("Unrecognized binary operator: %v", op)
		return nil, nil
	}
}

func (e *evaluator) evalBinaryOperatorEquals(lhs *rt.Object, rhs *rt.Object) bool {
	if lhs == rhs {
		return true
	}
	if lhs.Type() == types.Bool && rhs.Type() == types.Bool {
		return lhs.BoolValue() == rhs.BoolValue()
	}
	if lhs.Type() == types.Number && rhs.Type() == types.Number {
		return lhs.NumberValue() == rhs.NumberValue()
	}
	if lhs.Type() == types.String && rhs.Type() == types.String {
		return lhs.StringValue() == rhs.StringValue()
	}
	if lhs.Type() == types.Null && rhs.Type() == types.Null {
		return true // all nulls are equal.
	}
	return false
}

func (e *evaluator) evalCastExpression(node *ast.CastExpression) (*rt.Object, *rt.Unwind) {
	// Evaluate the underlying expression.
	obj, uw := e.evalExpression(node.Expression)
	if uw != nil {
		return nil, uw
	}

	// All bad static casts have been rejected, so we now need to check the runtime types.
	from := obj.Type()
	to := e.ctx.RequireType(node)
	if !types.CanConvert(from, to) {
		return nil, e.NewInvalidCastException(node, from, to)
	}

	return obj, nil
}

func (e *evaluator) evalIsInstExpression(node *ast.IsInstExpression) (*rt.Object, *rt.Unwind) {
	// Evaluate the underlying expression.
	obj, uw := e.evalExpression(node.Expression)
	if uw != nil {
		return nil, uw
	}

	// Now check the type and produce a bool object indicating whether the type is good.
	from := obj.Type()
	to := e.ctx.LookupType(node.Type)
	isinst := types.CanConvert(from, to)
	return rt.Bools[isinst], nil
}

func (e *evaluator) evalTypeOfExpression(node *ast.TypeOfExpression) (*rt.Object, *rt.Unwind) {
	// Evaluate the underlying expression.
	obj, uw := e.evalExpression(node.Expression)
	if uw != nil {
		return nil, uw
	}

	// Now just return the underlying type token for the object as a string.
	tok := obj.Type().Token()
	return rt.NewStringObject(string(tok)), nil
}

func (e *evaluator) evalConditionalExpression(node *ast.ConditionalExpression) (*rt.Object, *rt.Unwind) {
	// Evaluate the branches explicitly based on the result of the condition node.
	cond, uw := e.requireExpressionValue(node.Condition)
	if uw != nil {
		return nil, uw
	}
	if cond.BoolValue() {
		return e.evalExpression(node.Consequent)
	}
	return e.evalExpression(node.Alternate)
}

func (e *evaluator) evalSequenceExpression(node *ast.SequenceExpression) (*rt.Object, *rt.Unwind) {
	// Evaluate the sequence's prelude and then, afterwards, evaluate to its value.  If an unwind happens anywhere
	// during the prelude, we will abruptly terminate the sequence and return it.
	for _, prelnode := range node.Prelude {
		switch n := prelnode.(type) {
		case ast.Expression:
			if _, uw := e.evalExpression(n); uw != nil {
				return nil, uw
			}
		case ast.Statement:
			if uw := e.evalStatement(n); uw != nil {
				return nil, uw
			}
		}
	}
	return e.evalExpression(node.Value)
}
