// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"reflect"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/binder"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Interpreter can evaluate compiled MuPackages.
type Interpreter interface {
	core.Phase

	Ctx() *binder.Context // the binding context object.

	// EvaluatePackage performs evaluation on the given blueprint package.
	EvaluatePackage(pkg *symbols.Package, args core.Args) graph.Graph
	// EvaluateModule performs evaluation on the given module's entrypoint function.
	EvaluateModule(mod *symbols.Module, args core.Args) graph.Graph
	// EvaluateFunction performs an evaluation of the given function, using the provided arguments, returning its graph.
	EvaluateFunction(fnc symbols.Function, this *Object, args core.Args) graph.Graph
}

// New creates an interpreter that can be used to evaluate MuPackages.
func New(ctx *binder.Context) Interpreter {
	e := &evaluator{
		ctx:        ctx,
		globals:    make(globalMap),
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
	globals    globalMap        // the object values for global variable symbols.
	locals     *localScope      // local variable values scoped by the lexical structure.
	modinits   modinitMap       // a map of which modules have been initialized already.
	classinits classinitMap     // a map of which classes have been initialized already.
}

type globalMap map[symbols.Variable]*Reference
type modinitMap map[*symbols.Module]bool
type classinitMap map[*symbols.Class]bool

var _ Interpreter = (*evaluator)(nil)

func (e *evaluator) Ctx() *binder.Context { return e.ctx }
func (e *evaluator) Diag() diag.Sink      { return e.ctx.Diag }

// EvaluatePackage performs evaluation on the given blueprint package.
func (e *evaluator) EvaluatePackage(pkg *symbols.Package, args core.Args) graph.Graph {
	glog.Infof("Evaluating package '%v'", pkg.Name())
	if glog.V(2) {
		defer glog.V(2).Infof("Evaluation of package '%v' completed w/ %v warnings and %v errors",
			pkg.Name(), e.Diag().Warnings(), e.Diag().Errors())
	}

	// Search the package for a default module "index" to evaluate.
	for _, mod := range pkg.Modules {
		if mod.Default() {
			return e.EvaluateModule(mod, args)
		}
	}

	e.Diag().Errorf(errors.ErrorPackageHasNoDefaultModule.At(pkg.Tree()), pkg.Name())
	return nil
}

// EvaluateModule performs evaluation on the given module's entrypoint function.
func (e *evaluator) EvaluateModule(mod *symbols.Module, args core.Args) graph.Graph {
	glog.Infof("Evaluating module '%v'", mod.Token())
	if glog.V(2) {
		defer glog.V(2).Infof("Evaluation of module '%v' completed w/ %v warnings and %v errors",
			mod.Token(), e.Diag().Warnings(), e.Diag().Errors())
	}

	// Fetch the module's entrypoint function, erroring out if it doesn't have one.
	if ep, has := mod.Members[tokens.EntryPointFunction]; has {
		if epfnc, ok := ep.(symbols.Function); ok {
			return e.EvaluateFunction(epfnc, nil, args)
		}
	}

	e.Diag().Errorf(errors.ErrorModuleHasNoEntryPoint.At(mod.Tree()), mod.Name())
	return nil
}

// EvaluateFunction performs an evaluation of the given function, using the provided arguments, returning its graph.
func (e *evaluator) EvaluateFunction(fnc symbols.Function, this *Object, args core.Args) graph.Graph {
	glog.Infof("Evaluating function '%v'", fnc.Token())
	if glog.V(2) {
		defer glog.V(2).Infof("Evaluation of function '%v' completed w/ %v warnings and %v errors",
			fnc.Token(), e.Diag().Warnings(), e.Diag().Errors())
	}

	// Ensure that initializers have been run.
	switch f := fnc.(type) {
	case *symbols.ClassMethod:
		e.ensureClassInit(f.Parent)
	case *symbols.ModuleMethod:
		e.ensureModuleInit(f.Parent)
	default:
		contract.Failf("Unrecognized function evaluation type: %v", reflect.TypeOf(f))
	}

	// First, validate any arguments, and turn them into real runtime *Objects.
	var argos []*Object
	params := fnc.FuncNode().GetParameters()
	if params == nil {
		if len(args) != 0 {
			e.Diag().Errorf(errors.ErrorFunctionArgMismatch.At(fnc.Tree()), 0, len(args))
		}
	} else {
		if len(*params) != len(args) {
			e.Diag().Errorf(errors.ErrorFunctionArgMismatch.At(fnc.Tree()), 0, len(args))
		}

		ptys := fnc.FuncType().Parameters
		found := make(map[tokens.Name]bool)
		for i, param := range *params {
			pname := param.Name.Ident
			if arg, has := args[pname]; has {
				found[pname] = true
				argo := NewConstantObject(arg)
				if types.CanConvert(argo.Type, ptys[i]) {
					argos = append(argos, argo)
				} else {
					e.Diag().Errorf(errors.ErrorFunctionArgIncorrectType.At(fnc.Tree()), ptys[i], argo.Type)
					return nil
				}
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

	if e.Diag().Success() {
		// If the arguments bound correctly, make the call.
		_, uw := e.evalCall(fnc, this, argos...)
		if uw != nil {
			// If the call had an unwind out of it, then presumably we have an unhandled exception.
			e.issueUnhandledException(uw, errors.ErrorUnhandledException.At(fnc.Tree()))
		}
	}

	// TODO: turn the returned object into a graph.
	return nil
}

// Utility functions

// ensureClassInit ensures that the target's class initializer has been run.
func (e *evaluator) ensureClassInit(class *symbols.Class) {
	already := e.classinits[class]
	e.classinits[class] = true // set true before running, in case of cycles.

	if !already {
		// First ensure the module initializer has run.
		e.ensureModuleInit(class.Parent)

		// Next, run the class if it has an initializer.
		if init := class.GetInit(); init != nil {
			glog.V(7).Infof("Initializing class: %v", class)
			contract.Assert(len(init.Ty.Parameters) == 0)
			contract.Assert(init.Ty.Return == nil)
			obj, uw := e.evalCall(init, nil)
			contract.Assert(obj != nil)
			if uw != nil {
				// Must be an unhandled exception; spew it as an error (but keep going).
				e.issueUnhandledException(uw, errors.ErrorUnhandledInitException.At(init.Tree()), class)
			}
		} else {
			glog.V(7).Infof("Class has no initializer: %v", class)
		}
	}
}

// ensureModuleInit ensures that the target's module initializer has been run.  It also evaluates dependency module
// initializers, assuming they have been declared.  If they have not, those will run when we access them.
func (e *evaluator) ensureModuleInit(mod *symbols.Module) {
	already := e.modinits[mod]
	e.modinits[mod] = true // set true before running, in case of cycles.

	if !already {
		// First ensure all imported module initializers are run, in the order in which they were given.
		for _, imp := range mod.Imports {
			e.ensureModuleInit(imp)
		}

		// Next, run the module initializer if it has one.
		if init := mod.GetInit(); init != nil {
			glog.V(7).Infof("Initializing module: %v", mod)
			contract.Assert(len(init.Type.Parameters) == 0)
			contract.Assert(init.Type.Return == nil)
			obj, uw := e.evalCall(init, nil)
			contract.Assert(obj != nil)
			if uw != nil {
				// Must be an unhandled exception; spew it as an error (but keep going).
				e.issueUnhandledException(uw, errors.ErrorUnhandledInitException.At(init.Tree()), mod)
			}
		} else {
			glog.V(7).Infof("Module has no initializer: %v", mod)
		}
	}
}

// issueUnhandledException issues an unhandled exception error using the given diagnostic and unwind information.
func (e *evaluator) issueUnhandledException(uw *Unwind, err *diag.Diag, args ...interface{}) {
	contract.Assert(uw.Throw())
	var msg string
	if thrown := uw.Thrown(); thrown != nil {
		msg = thrown.Data.(string)
	}
	if msg == "" {
		msg = "no details available"
	}
	args = append(args, msg)
	// TODO: ideally we would also have a stack trace to show here.
	e.Diag().Errorf(err, args...)
}

// pushScope pushes a new local and context scope.  The frame argument indicates whether this is an activation frame,
// meaning that searches for local variables will not probe into parent scopes (since they are inaccessible).
func (e *evaluator) pushScope(frame bool) {
	e.locals.Push(frame) // pushing the local scope also updates the context scope.
}

// popScope pops the current local and context scopes.
func (e *evaluator) popScope() {
	e.locals.Pop() // popping the local scope also updates the context scope.
}

// Functions

func (e *evaluator) evalCall(fnc symbols.Function, this *Object, args ...*Object) (*Object, *Unwind) {
	glog.V(7).Infof("Evaluating call to fnc %v; this=%v args=%v", fnc, this != nil, len(args))

	// Save the prior func, set the new one, and restore upon exit.
	prior := fnc
	e.fnc = fnc
	defer func() { e.fnc = prior }()

	// Set up a new lexical scope "activation frame" in which we can bind the parameters; restore it upon exit.
	e.pushScope(true)
	defer e.popScope()

	// If the target is an instance method, the "this" and "super" variables must be bound to values.
	if this != nil {
		switch f := fnc.(type) {
		case *symbols.ClassMethod:
			contract.Assertf(!f.Static(), "Static methods don't have 'this' arguments, but we got a non-nil one")
			contract.Assertf(types.CanConvert(this.Type, f.Parent), "'this' argument was of the wrong type")
			e.ctx.Scope.Register(f.Parent.This)
			e.locals.InitValueReference(f.Parent.This, &Reference{obj: this, readonly: true})
			if f.Parent.Super != nil {
				e.ctx.Scope.Register(f.Parent.Super)
				e.locals.InitValueReference(f.Parent.Super, &Reference{obj: this, readonly: true})
			}
		default:
			contract.Failf("Only class methods should have 'this' arguments, but we got a non-nil one")
		}
	} else {
		// If no this was supplied, we expect that this is either not a class method, or it is a static.
		switch f := fnc.(type) {
		case *symbols.ClassMethod:
			contract.Assertf(f.Static(), "Instance methods require 'this' arguments, but we got nil")
		}
	}

	// Ensure that the arguments line up to the parameter slots and add them to the frame.
	node := fnc.FuncNode()
	params := node.GetParameters()
	if params == nil {
		contract.Assert(len(args) == 0)
	} else {
		contract.Assert(len(args) == len(*params))
		for i, param := range *params {
			sym := e.ctx.RequireVariable(param).(*symbols.LocalVariable)
			e.ctx.Scope.Register(sym)
			arg := args[i]
			contract.Assert(types.CanConvert(arg.Type, sym.Type()))
			e.locals.SetValue(sym, arg)
		}
	}

	// Now perform the invocation by visiting the body.
	uw := e.evalBlock(node.GetBody())

	// Check that the unwind is as expected.  In particular:
	//     1) no breaks or continues are expected;
	//     2) any throw is treated as an unhandled exception that propagates to the caller.
	//     3) any return is checked to be of the expected type, and returned as the result of the call.
	retty := fnc.FuncType().Return
	if uw != nil {
		if uw.Throw() {
			return nil, uw
		}

		contract.Assert(uw.Return()) // break/continue not expected.
		ret := uw.Returned()
		contract.Assert((retty == nil) == (ret == nil))
		contract.Assert(ret == nil || types.CanConvert(ret.Type, retty))
		return ret, nil
	}

	// An absence of a return is okay for void-returning functions.
	contract.Assert(retty == nil)
	return nil, nil
}

// Statements

func (e *evaluator) evalStatement(node ast.Statement) *Unwind {
	// Simply switch on the node type and dispatch to the specific function, returning the Unwind info.
	switch n := node.(type) {
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
	case *ast.LabeledStatement:
		return e.evalLabeledStatement(n)
	case *ast.ReturnStatement:
		return e.evalReturnStatement(n)
	case *ast.ThrowStatement:
		return e.evalThrowStatement(n)
	case *ast.WhileStatement:
		return e.evalWhileStatement(n)
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

func (e *evaluator) evalBlock(node *ast.Block) *Unwind {
	// Push a scope at the start, and pop it at afterwards; both for the symbol context and local variable values.
	e.pushScope(false)
	defer e.popScope()

	for _, stmt := range node.Statements {
		if uw := e.evalStatement(stmt); uw != nil {
			return uw
		}
	}

	return nil
}

func (e *evaluator) evalLocalVariableDeclaration(node *ast.LocalVariableDeclaration) *Unwind {
	// Populate the variable in the scope.
	sym := e.ctx.RequireVariable(node.Local).(*symbols.LocalVariable)
	e.ctx.Scope.Register(sym)

	// If there is a default value, set it now.
	if node.Local.Default != nil {
		obj := NewConstantObject(*node.Local.Default)
		e.locals.SetValue(sym, obj)
	}

	return nil
}

func (e *evaluator) evalTryCatchFinally(node *ast.TryCatchFinally) *Unwind {
	// First, execute the TryBlock.
	uw := e.evalBlock(node.TryBlock)
	if uw != nil && uw.Throw() {
		// The try block threw something; see if there is a handler that covers this.
		thrown := uw.Thrown()
		if node.CatchBlocks != nil {
			for _, catch := range *node.CatchBlocks {
				ex := e.ctx.RequireVariable(catch.Exception).(*symbols.LocalVariable)
				exty := ex.Type()
				contract.Assert(types.CanConvert(exty, types.Error))
				if types.CanConvert(thrown.Type, exty) {
					// This type matched, so this handler will catch the exception.  Set the exception variable,
					// evaluate the block, and swap the Unwind information (thereby "handling" the in-flight exception).
					e.pushScope(false)
					e.locals.SetValue(ex, thrown)
					uw = e.evalBlock(catch.Block)
					e.popScope()
					break
				}
			}
		}
	}

	// No matter the Unwind instructions, be sure to invoke the FinallyBlock.
	if node.FinallyBlock != nil {
		uwf := e.evalBlock(node.FinallyBlock)

		// Any Unwind information from the finally block overrides the try Unwind that was in flight.
		if uwf != nil {
			uw = uwf
		}
	}

	return uw
}

func (e *evaluator) evalBreakStatement(node *ast.BreakStatement) *Unwind {
	var label *tokens.Name
	if node.Label != nil {
		label = &node.Label.Ident
	}
	return NewBreakUnwind(label)
}

func (e *evaluator) evalContinueStatement(node *ast.ContinueStatement) *Unwind {
	var label *tokens.Name
	if node.Label != nil {
		label = &node.Label.Ident
	}
	return NewContinueUnwind(label)
}

func (e *evaluator) evalIfStatement(node *ast.IfStatement) *Unwind {
	// Evaluate the branches explicitly based on the result of the condition node.
	cond, uw := e.evalExpression(node.Condition)
	if uw != nil {
		return uw
	}
	if cond.Bool() {
		return e.evalStatement(node.Consequent)
	} else if node.Alternate != nil {
		return e.evalStatement(*node.Alternate)
	}
	return nil
}

func (e *evaluator) evalLabeledStatement(node *ast.LabeledStatement) *Unwind {
	// Evaluate the underlying statement; if it is breaking or continuing to this label, stop the Unwind.
	uw := e.evalStatement(node.Statement)
	if uw != nil && uw.Label() != nil && *uw.Label() == node.Label.Ident {
		contract.Assert(uw.Continue() || uw.Break())
		// TODO: perform correct break/continue behavior when the label is affixed to a loop.
		uw = nil
	}
	return uw
}

func (e *evaluator) evalReturnStatement(node *ast.ReturnStatement) *Unwind {
	var ret *Object
	if node.Expression != nil {
		var uw *Unwind
		if ret, uw = e.evalExpression(*node.Expression); uw != nil {
			// If the expression caused an Unwind, propagate that and ignore the returned object.
			return uw
		}
	}
	return NewReturnUnwind(ret)
}

func (e *evaluator) evalThrowStatement(node *ast.ThrowStatement) *Unwind {
	thrown, uw := e.evalExpression(node.Expression)
	if uw != nil {
		// If the throw expression itself threw an exception, propagate that instead.
		return uw
	}
	contract.Assert(thrown != nil)
	return NewThrowUnwind(thrown)
}

func (e *evaluator) evalWhileStatement(node *ast.WhileStatement) *Unwind {
	// So long as the test evaluates to true, keep on visiting the body.
	var uw *Unwind
	for {
		test, uw := e.evalExpression(node.Test)
		if uw != nil {
			return uw
		}
		if test.Bool() {
			if uws := e.evalStatement(node.Body); uw != nil {
				if uws.Continue() {
					contract.Assertf(uws.Label() == nil, "Labeled continue not yet supported")
					continue
				} else if uws.Break() {
					contract.Assertf(uws.Label() == nil, "Labeled break not yet supported")
					break
				} else {
					// If it's not a continue or break, stash the Unwind away and return it.
					uw = uws
					break
				}
			}
		} else {
			break
		}
	}
	return uw // usually nil, unless a body statement threw/returned.
}

func (e *evaluator) evalMultiStatement(node *ast.MultiStatement) *Unwind {
	for _, stmt := range node.Statements {
		if uw := e.evalStatement(stmt); uw != nil {
			return uw
		}
	}
	return nil
}

func (e *evaluator) evalExpressionStatement(node *ast.ExpressionStatement) *Unwind {
	// Just evaluate the expression, drop its object on the floor, and propagate its Unwind information.
	_, uw := e.evalExpression(node.Expression)
	return uw
}

// Expressions

func (e *evaluator) evalExpression(node ast.Expression) (*Object, *Unwind) {
	// Simply switch on the node type and dispatch to the specific function, returning the object and Unwind info.
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

// evalLValueExpression evaluates an expression for use as an l-value; in particular, this loads the target as a
// pointer/reference object, rather than as an ordinary value, so that it can be used in an assignment.  This is only
// valid on the subset of AST nodes that are legal l-values (very few of them, it turns out).
func (e *evaluator) evalLValueExpression(node ast.Expression) (*Object, *Unwind) {
	switch n := node.(type) {
	case *ast.LoadLocationExpression:
		return e.evalLoadLocationExpressionFor(n, true)
	case *ast.UnaryOperatorExpression:
		contract.Assert(n.Operator == ast.OpDereference)
		return e.evalUnaryOperatorExpressionFor(n, true)
	default:
		contract.Failf("Unrecognized l-value expression type: %v", node.GetKind())
		return nil, nil
	}
}

func (e *evaluator) evalNullLiteral(node *ast.NullLiteral) (*Object, *Unwind) {
	return NewPrimitiveObject(types.Null, nil), nil
}

func (e *evaluator) evalBoolLiteral(node *ast.BoolLiteral) (*Object, *Unwind) {
	return NewPrimitiveObject(types.Bool, node.Value), nil
}

func (e *evaluator) evalNumberLiteral(node *ast.NumberLiteral) (*Object, *Unwind) {
	return NewPrimitiveObject(types.Number, node.Value), nil
}

func (e *evaluator) evalStringLiteral(node *ast.StringLiteral) (*Object, *Unwind) {
	return NewPrimitiveObject(types.String, node.Value), nil
}

func (e *evaluator) evalArrayLiteral(node *ast.ArrayLiteral) (*Object, *Unwind) {
	// Fetch this expression type and assert that it's an array.
	ty := e.ctx.RequireType(node).(*symbols.ArrayType)

	// Now create the array data.
	var sz *int
	var arr []Data

	// If there's a node size, ensure it's a number, and initialize the array.
	if node.Size != nil {
		sze, uw := e.evalExpression(*node.Size)
		if uw != nil {
			return nil, uw
		}
		// TODO: this really ought to be an int, not a float...
		sz := int(sze.Number())
		if sz < 0 {
			// If the size is less than zero, raise a new error.
			return nil, NewThrowUnwind(NewErrorObject("Invalid array size (must be >= 0)"))
		}
		arr = make([]Data, sz)
	}

	// If there are elements, place them into the array.  This has two behaviors:
	//     1) if there is a size, there can be up to that number of elements, which are set;
	//     2) if there is no size, all of the elements are appended to the array.
	if node.Elements != nil {
		if sz == nil {
			// Right-size the array.
			arr = make([]Data, 0, len(*node.Elements))
		} else if len(*node.Elements) > *sz {
			// The element count exceeds the size; raise an error.
			return nil, NewThrowUnwind(
				NewErrorObject("Invalid number of array elements; expected <=%v, got %v",
					*sz, len(*node.Elements)))
		}

		for i, elem := range *node.Elements {
			expr, uw := e.evalExpression(elem)
			if uw != nil {
				return nil, uw
			}
			if sz == nil {
				arr = append(arr, expr)
			} else {
				arr[i] = expr
			}
		}
	}

	// Finally wrap the array data in a literal object.
	return NewPrimitiveObject(ty, arr), nil
}

func (e *evaluator) evalObjectLiteral(node *ast.ObjectLiteral) (*Object, *Unwind) {
	obj := NewObject(e.ctx.Types[node])

	if node.Properties != nil {
		// The binder already checked that the properties are legal, so we will simply store them as values.
		for _, init := range *node.Properties {
			val, uw := e.evalExpression(init.Value)
			if uw != nil {
				return nil, uw
			}
			member := init.Property.Tok.Name()
			obj.GetPropertyReference(member.Name(), true).Set(val)
		}
	}

	return obj, nil
}

func (e *evaluator) evalLoadLocationExpression(node *ast.LoadLocationExpression) (*Object, *Unwind) {
	return e.evalLoadLocationExpressionFor(node, false)
}

func (e *evaluator) evalLoadLocationExpressionFor(node *ast.LoadLocationExpression, lval bool) (*Object, *Unwind) {
	// If there's a 'this', evaluate it.
	var this *Object
	if node.Object != nil {
		var uw *Unwind
		if this, uw = e.evalExpression(*node.Object); uw != nil {
			return nil, uw
		}
	}

	// Create a pointer to the target location.
	var pv *Reference
	var ty symbols.Type
	tok := node.Name.Tok
	if this == nil && tok.Simple() {
		// If there is no object and the name is simple, it refers to a local variable in the current scope.
		loc := e.ctx.Scope.Lookup(tok.Name())
		contract.Assert(loc != nil)
		pv = e.locals.GetValueReference(loc, true)
		ty = loc.Type()
	} else {
		sym := e.ctx.RequireSymbolToken(tok)
		switch s := sym.(type) {
		case *symbols.ClassProperty:
			// Search the class's properties and, if not present, allocate a new one.
			contract.Assert(this != nil)
			pv = this.GetPropertyReference(sym.Name(), true)
			ty = s.Type()
		case *symbols.ClassMethod:
			// Create a new readonly ref slot, pointing to the method, that will abandon if overwritten.
			contract.Assert(this != nil)
			// TODO[marapongo/mu#56]: consider permitting "dynamic" method overwriting.
			pv = &Reference{
				obj:      NewFunctionObject(s, this),
				readonly: true,
			}
			ty = s.Type()
		case *symbols.ModuleProperty:
			// Search the globals table and, if not present, allocate a new property.
			contract.Assert(this == nil)
			ref, has := e.globals[s]
			if !has {
				ref = &Reference{}
				e.globals[s] = ref
			}
			pv = ref
			ty = s.Type()
		case *symbols.ModuleMethod:
			// Create a new readonly ref slot, pointing to the method, that will abandon if overwritten.
			contract.Assert(this == nil)
			// TODO[marapongo/mu#56]: consider permitting "dynamic" method overwriting.
			pv = &Reference{
				obj:      NewFunctionObject(s, nil),
				readonly: true,
			}
			ty = s.Type
		default:
			contract.Failf("Unexpected symbol token kind during load expression: %v", tok)
		}
	}

	// If this isn't for an l-value, return the raw object.  Otherwise, make sure it's not readonly, and return it.
	if lval {
		// A readonly reference cannot be used as an l-value.
		if pv.Readonly() {
			e.Diag().Errorf(errors.ErrorIllegalReadonlyLValue.At(node))
		}
		return NewReferenceObject(ty, pv), nil
	}
	return pv.Obj(), nil
}

func (e *evaluator) evalLoadDynamicExpression(node *ast.LoadDynamicExpression) (*Object, *Unwind) {
	contract.Failf("Evaluation of %v nodes not yet implemented", reflect.TypeOf(node))
	return nil, nil
}

func (e *evaluator) evalNewExpression(node *ast.NewExpression) (*Object, *Unwind) {
	// TODO: create a new object and invoke its constructor.
	contract.Failf("Evaluation of %v nodes not yet implemented", reflect.TypeOf(node))
	return nil, nil
}

func (e *evaluator) evalInvokeFunctionExpression(node *ast.InvokeFunctionExpression) (*Object, *Unwind) {
	// Evaluate the function that we are meant to invoke.
	fncobj, uw := e.evalExpression(node.Function)
	if uw != nil {
		return nil, uw
	}

	// Ensure that this actually led to a function; this is guaranteed by the binder.
	var fnc funcStub
	switch fncobj.Type.(type) {
	case *symbols.FunctionType:
		fnc = fncobj.Data.(funcStub)
		contract.Assert(fnc.Func != nil)
	default:
		contract.Failf("Expected function expression to yield a function type")
	}

	// Now evaluate the arguments to the function, in order.
	var args []*Object
	if node.Arguments != nil {
		for _, arg := range *node.Arguments {
			argobj, uw := e.evalExpression(arg)
			if uw != nil {
				return nil, uw
			}
			args = append(args, argobj)
		}
	}

	// Finally, actually dispatch the call; this will create the activation frame, etc. for us.
	return e.evalCall(fnc.Func, fnc.This, args...)
}

func (e *evaluator) evalLambdaExpression(node *ast.LambdaExpression) (*Object, *Unwind) {
	// TODO: create the lambda object that can be invoked at runtime.
	contract.Failf("Evaluation of %v nodes not yet implemented", reflect.TypeOf(node))
	return nil, nil
}

func (e *evaluator) evalUnaryOperatorExpression(node *ast.UnaryOperatorExpression) (*Object, *Unwind) {
	return e.evalUnaryOperatorExpressionFor(node, false)
}

func (e *evaluator) evalUnaryOperatorExpressionFor(node *ast.UnaryOperatorExpression, lval bool) (*Object, *Unwind) {
	contract.Assertf(!lval || node.Operator == ast.OpDereference, "Only dereference unary ops support l-values")

	// Evaluate the operand and prepare to use it.
	var opand *Object
	if node.Operator == ast.OpAddressof ||
		node.Operator == ast.OpPlusPlus || node.Operator == ast.OpMinusMinus {
		// These operators require an l-value; so we bind the expression a bit differently.
		var uw *Unwind
		if opand, uw = e.evalLValueExpression(node.Operand); uw != nil {
			return nil, uw
		}
	} else {
		// Otherwise, we just need to evaluate the operand as usual.
		var uw *Unwind
		if opand, uw = e.evalExpression(node.Operand); uw != nil {
			return nil, uw
		}
	}

	// Now switch on the operator and perform its specific operation.
	switch node.Operator {
	case ast.OpDereference:
		// The target is a pointer.  If this is for an l-value, just return it as-is; otherwise, dereference it.
		ref := opand.Reference()
		contract.Assert(ref != nil)
		if lval {
			return opand, nil
		}
		return ref.Obj(), nil
	case ast.OpAddressof:
		// The target is an l-value, load its address.
		contract.Assert(opand.Reference() != nil)
		return opand, nil
	case ast.OpUnaryPlus:
		// The target is a number; simply fetch it (asserting its value), and + it.
		return NewNumberObject(+opand.Number()), nil
	case ast.OpUnaryMinus:
		// The target is a number; simply fetch it (asserting its value), and - it.
		return NewNumberObject(-opand.Number()), nil
	case ast.OpLogicalNot:
		// The target is a boolean; simply fetch it (asserting its value), and ! it.
		return NewBoolObject(!opand.Bool()), nil
	case ast.OpBitwiseNot:
		// The target is a number; simply fetch it (asserting its value), and ^ it (similar to C's ~ operator).
		return NewNumberObject(float64(^int64(opand.Number()))), nil
	case ast.OpPlusPlus:
		// The target is an l-value; we must load it, ++ it, and return the appropriate prefix/postfix value.
		ref := opand.Reference()
		old := ref.Obj()
		val := old.Number()
		new := NewNumberObject(val + 1)
		ref.Set(new)
		if node.Postfix {
			return old, nil
		} else {
			return new, nil
		}
	case ast.OpMinusMinus:
		// The target is an l-value; we must load it, -- it, and return the appropriate prefix/postfix value.
		ref := opand.Reference()
		old := ref.Obj()
		val := old.Number()
		new := NewNumberObject(val - 1)
		ref.Set(new)
		if node.Postfix {
			return old, nil
		} else {
			return new, nil
		}
	default:
		contract.Failf("Unrecognized unary operator: %v", node.Operator)
		return nil, nil
	}
}

func (e *evaluator) evalBinaryOperatorExpression(node *ast.BinaryOperatorExpression) (*Object, *Unwind) {
	// TODO: perform the binary operator's behavior.
	contract.Failf("Evaluation of %v nodes not yet implemented", reflect.TypeOf(node))
	return nil, nil
}

func (e *evaluator) evalCastExpression(node *ast.CastExpression) (*Object, *Unwind) {
	contract.Failf("Evaluation of %v nodes not yet implemented", reflect.TypeOf(node))
	return nil, nil
}

func (e *evaluator) evalIsInstExpression(node *ast.IsInstExpression) (*Object, *Unwind) {
	contract.Failf("Evaluation of %v nodes not yet implemented", reflect.TypeOf(node))
	return nil, nil
}

func (e *evaluator) evalTypeOfExpression(node *ast.TypeOfExpression) (*Object, *Unwind) {
	contract.Failf("Evaluation of %v nodes not yet implemented", reflect.TypeOf(node))
	return nil, nil
}

func (e *evaluator) evalConditionalExpression(node *ast.ConditionalExpression) (*Object, *Unwind) {
	// Evaluate the branches explicitly based on the result of the condition node.
	cond, uw := e.evalExpression(node.Condition)
	if uw != nil {
		return nil, uw
	}
	if cond.Bool() {
		return e.evalExpression(node.Consequent)
	}
	return e.evalExpression(node.Alternate)
}

func (e *evaluator) evalSequenceExpression(node *ast.SequenceExpression) (*Object, *Unwind) {
	// Simply walk through the sequence and return the last object.
	var obj *Object
	contract.Assert(len(node.Expressions) > 0)
	for _, expr := range node.Expressions {
		var uw *Unwind
		if obj, uw = e.evalExpression(expr); uw != nil {
			// If the Unwind was non-nil, stop visiting the expressions and propagate it now.
			return nil, uw
		}
	}
	// Return the last expression's object.
	return obj, nil
}
