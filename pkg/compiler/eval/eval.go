// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/binder"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Interpreter can evaluate compiled MuPackages.
type Interpreter interface {
	core.Phase

	Ctx() *binder.Context // the binding context object.

	// EvaluatePackage performs evaluation on the given blueprint package, starting in its entrypoint.
	EvaluatePackage(pkg *pack.Package) graph.Graph
}

// New creates an interpreter that can be used to evaluate MuPackages.
func New(ctx *binder.Context) Interpreter {
	e := &evaluator{
		ctx:     ctx,
		globals: make(globalMap),
	}
	initLocalScope(&e.locals)
	return e
}

type evaluator struct {
	fnc     *symbols.ModuleMethod // the function under evaluation.
	ctx     *binder.Context       // the binding context with type and symbol information.
	globals globalMap             // the object values for global variable symbols.
	locals  *localScope           // local variable values scoped by the lexical structure.
}

type globalMap map[symbols.Variable]*Object

var _ Interpreter = (*evaluator)(nil)

func (e *evaluator) Ctx() *binder.Context { return e.ctx }
func (e *evaluator) Diag() diag.Sink      { return e.ctx.Diag }

func (e *evaluator) EvaluatePackage(pkg *pack.Package) graph.Graph {
	// TODO: find the entrypoint.
	// TODO: pair up the ctx args, if any, with the entrypoint's parameters.
	// TODO: visit the function.
	return nil
}

// Utility functions

func (e *evaluator) pushScope() {
	e.locals.Push()   // for local variables
	e.ctx.Scope.Pop() // for symbol bindings
}

func (e *evaluator) popScope() {
	e.locals.Pop()    // for local variables.
	e.ctx.Scope.Pop() // for symbol bindings.
}

// Statements

func (e *evaluator) evalStatement(node ast.Statement) *unwind {
	// Simply switch on the node type and dispatch to the specific function, returning the unwind info.
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

func (e *evaluator) evalBlock(node *ast.Block) *unwind {
	// Push a scope at the start, and pop it at afterwards; both for the symbol context and local variable values.
	e.pushScope()
	defer e.popScope()

	for _, stmt := range node.Statements {
		if uw := e.evalStatement(stmt); uw != nil {
			return uw
		}
	}

	return nil
}

func (e *evaluator) evalLocalVariableDeclaration(node *ast.LocalVariableDeclaration) *unwind {
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

func (e *evaluator) evalTryCatchFinally(node *ast.TryCatchFinally) *unwind {
	// First, execute the TryBlock.
	uw := e.evalBlock(node.TryBlock)
	if uw != nil && uw.Thrown != nil {
		// The try block threw something; see if there is a handler that covers this.
		if node.CatchBlocks != nil {
			for _, catch := range *node.CatchBlocks {
				ex := e.ctx.RequireVariable(catch.Exception).(*symbols.LocalVariable)
				exty := ex.Type()
				contract.Assert(types.CanConvert(exty, types.Error))
				if types.CanConvert(uw.Thrown.Type, exty) {
					// This type matched, so this handler will catch the exception.  Set the exception variable,
					// evaluate the block, and swap the unwind information (thereby "handling" the in-flight exception).
					e.pushScope()
					e.locals.SetValue(ex, uw.Thrown)
					uw = e.evalBlock(catch.Block)
					e.popScope()
					break
				}
			}
		}
	}

	// No matter the unwind instructions, be sure to invoke the FinallyBlock.
	if node.FinallyBlock != nil {
		uwf := e.evalBlock(node.FinallyBlock)

		// Any unwind information from the finally block overrides the try unwind that was in flight.
		if uwf != nil {
			uw = uwf
		}
	}

	return uw
}

func (e *evaluator) evalBreakStatement(node *ast.BreakStatement) *unwind {
	var label *tokens.Name
	if node.Label != nil {
		label = &node.Label.Ident
	}
	return breakUnwind(label)
}

func (e *evaluator) evalContinueStatement(node *ast.ContinueStatement) *unwind {
	var label *tokens.Name
	if node.Label != nil {
		label = &node.Label.Ident
	}
	return continueUnwind(label)
}

func (e *evaluator) evalIfStatement(node *ast.IfStatement) *unwind {
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

func (e *evaluator) evalLabeledStatement(node *ast.LabeledStatement) *unwind {
	// Evaluate the underlying statement; if it is breaking or continuing to this label, stop the unwind.
	uw := e.evalStatement(node.Statement)
	if uw != nil && uw.Label != nil && *uw.Label == node.Label.Ident {
		contract.Assert(uw.Return == false)
		contract.Assert(uw.Throw == false)
		// TODO: perform correct break/continue behavior when the label is affixed to a loop.
		uw = nil
	}
	return uw
}

func (e *evaluator) evalReturnStatement(node *ast.ReturnStatement) *unwind {
	var ret *Object
	if node.Expression != nil {
		var uw *unwind
		if ret, uw = e.evalExpression(*node.Expression); uw != nil {
			// If the expression caused an unwind, propagate that and ignore the returned object.
			return uw
		}
	}
	return returnUnwind(ret)
}

func (e *evaluator) evalThrowStatement(node *ast.ThrowStatement) *unwind {
	thrown, uw := e.evalExpression(node.Expression)
	if uw != nil {
		// If the throw expression itself threw an exception, propagate that instead.
		return uw
	}
	contract.Assert(thrown != nil)
	return throwUnwind(thrown)
}

func (e *evaluator) evalWhileStatement(node *ast.WhileStatement) *unwind {
	// So long as the test evaluates to true, keep on visiting the body.
	var uw *unwind
	for {
		test, uw := e.evalExpression(node.Test)
		if uw != nil {
			return uw
		}
		if test.Bool() {
			if uws := e.evalStatement(node.Body); uw != nil {
				if uws.Continue {
					contract.Assertf(uws.Label == nil, "Labeled continue not yet supported")
					continue
				} else if uws.Break {
					contract.Assertf(uws.Label == nil, "Labeled break not yet supported")
					break
				} else {
					// If it's not a continue or break, stash the unwind away and return it.
					uw = uws
				}
			}
		} else {
			break
		}
	}
	return uw // usually nil, unless a body statement threw/returned.
}

func (e *evaluator) evalMultiStatement(node *ast.MultiStatement) *unwind {
	for _, stmt := range node.Statements {
		if uw := e.evalStatement(stmt); uw != nil {
			return uw
		}
	}
	return nil
}

func (e *evaluator) evalExpressionStatement(node *ast.ExpressionStatement) *unwind {
	// Just evaluate the expression, drop its object on the floor, and propagate its unwind information.
	_, uw := e.evalExpression(node.Expression)
	return uw
}

// Expressions

func (e *evaluator) evalExpression(node ast.Expression) (*Object, *unwind) {
	// Simply switch on the node type and dispatch to the specific function, returning the object and unwind info.
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

func (e *evaluator) evalNullLiteral(node *ast.NullLiteral) (*Object, *unwind) {
	return NewPrimitiveObject(types.Null, nil), nil
}

func (e *evaluator) evalBoolLiteral(node *ast.BoolLiteral) (*Object, *unwind) {
	return NewPrimitiveObject(types.Bool, node.Value), nil
}

func (e *evaluator) evalNumberLiteral(node *ast.NumberLiteral) (*Object, *unwind) {
	return NewPrimitiveObject(types.Number, node.Value), nil
}

func (e *evaluator) evalStringLiteral(node *ast.StringLiteral) (*Object, *unwind) {
	return NewPrimitiveObject(types.String, node.Value), nil
}

func (e *evaluator) evalArrayLiteral(node *ast.ArrayLiteral) (*Object, *unwind) {
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
			return nil, throwUnwind(NewErrorObject("Invalid array size (must be >= 0)"))
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
			return nil, throwUnwind(
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

func (e *evaluator) evalObjectLiteral(node *ast.ObjectLiteral) (*Object, *unwind) {
	// TODO: create a new object value.
	return nil, nil
}

func (e *evaluator) evalLoadLocationExpression(node *ast.LoadLocationExpression) (*Object, *unwind) {
	// TODO: create a pointer to the given location.
	return nil, nil
}

func (e *evaluator) evalLoadDynamicExpression(node *ast.LoadDynamicExpression) (*Object, *unwind) {
	return nil, nil
}

func (e *evaluator) evalNewExpression(node *ast.NewExpression) (*Object, *unwind) {
	// TODO: create a new object and invoke its constructor.
	return nil, nil
}

func (e *evaluator) evalInvokeFunctionExpression(node *ast.InvokeFunctionExpression) (*Object, *unwind) {
	// TODO: resolve the target to a function, set up an activation record, and invoke it.
	return nil, nil
}

func (e *evaluator) evalLambdaExpression(node *ast.LambdaExpression) (*Object, *unwind) {
	// TODO: create the lambda object that can be invoked at runtime.
	return nil, nil
}

func (e *evaluator) evalUnaryOperatorExpression(node *ast.UnaryOperatorExpression) (*Object, *unwind) {
	// TODO: perform the unary operator's behavior.
	return nil, nil
}

func (e *evaluator) evalBinaryOperatorExpression(node *ast.BinaryOperatorExpression) (*Object, *unwind) {
	// TODO: perform the binary operator's behavior.
	return nil, nil
}

func (e *evaluator) evalCastExpression(node *ast.CastExpression) (*Object, *unwind) {
	return nil, nil
}

func (e *evaluator) evalIsInstExpression(node *ast.IsInstExpression) (*Object, *unwind) {
	return nil, nil
}

func (e *evaluator) evalTypeOfExpression(node *ast.TypeOfExpression) (*Object, *unwind) {
	return nil, nil
}

func (e *evaluator) evalConditionalExpression(node *ast.ConditionalExpression) (*Object, *unwind) {
	// Evaluate the branches explicitly based on the result of the condition node.
	cond, uw := e.evalExpression(node.Condition)
	if uw != nil {
		return nil, uw
	}
	if cond.Bool() {
		return e.evalExpression(node.Consequent)
	} else {
		return e.evalExpression(node.Alternate)
	}
}

func (e *evaluator) evalSequenceExpression(node *ast.SequenceExpression) (*Object, *unwind) {
	// Simply walk through the sequence and return the last object.
	var obj *Object
	contract.Assert(len(node.Expressions) > 0)
	for _, expr := range node.Expressions {
		var uw *unwind
		if obj, uw = e.evalExpression(expr); uw != nil {
			// If the unwind was non-nil, stop visiting the expressions and propagate it now.
			return nil, uw
		}
	}
	// Return the last expression's object.
	return obj, nil
}
