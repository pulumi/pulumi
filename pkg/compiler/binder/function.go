// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// bindFunctionBody binds a function body, including a scope, its parameters, and its expressions and statements.
func (b *binder) bindFunctionBody(node ast.Function) {
	// Enter a new scope, bind the parameters, and then bind the body using a visitor.
	scope := b.scope.Push()
	defer scope.Pop()
	params := node.GetParameters()
	if params != nil {
		for _, param := range *params {
			// Register this variable's type and associate its name with the identifier.
			b.registerVariableType(param)
			b.scope.MustRegister(symbols.NewLocalVariableSym(param))
		}
	}

	body := node.GetBody()
	if body != nil {
		v := newASTBinder(b, node)
		ast.Walk(v, body)
	}
}

// astBinder is an AST visitor implementation that understands how to deal with all sorts of node types.  It
// does not visit children, however, as it relies on the depth-first order walk supplied by the AST package.  The
// overall purpose of this is to perform validation, and record types and symbols that're needed during evaluation.
type astBinder struct {
	b      *binder
	fnc    ast.Function                  // the current function.
	labels map[tokens.Name]ast.Statement // a map of known labels (for validation of jumps).
}

func newASTBinder(b *binder, fnc ast.Function) ast.Visitor {
	return &astBinder{
		b:      b,
		fnc:    fnc,
		labels: make(map[tokens.Name]ast.Statement),
	}
}

var _ ast.Visitor = (*astBinder)(nil)

func (a *astBinder) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	// Statements
	case *ast.Block:
		// Entering a new block requires a fresh lexical scope.
		a.b.scope.Push()
	case *ast.LocalVariable:
		// Encountering a new local variable results in registering it; both to the type and symbol table.
		a.b.registerVariableType(n)
		a.b.scope.MustRegister(symbols.NewLocalVariableSym(n))
	case *ast.LabeledStatement:
		// Ensure this label doesn't already exist and then register it.
		label := tokens.Name(n.Label.Ident)
		if other, has := a.labels[label]; has {
			a.b.Diag().Errorf(errors.ErrorDuplicateLabel.At(n), label, other)
		}
		a.labels[label] = n
	}

	// Return the current visitor to keep on visitin'.
	return a
}

func (a *astBinder) After(node ast.Node) {
	switch n := node.(type) {
	// Statements
	case *ast.Block:
		// Exiting a block restores the prior lexical context.
		a.b.scope.Pop()
	case *ast.BreakStatement:
		// If the break specifies a label, ensure that it exists.
		if n.Label != nil {
			label := tokens.Name(n.Label.Ident)
			if _, has := a.labels[label]; !has {
				a.b.Diag().Errorf(errors.ErrorUnknownJumpLabel.At(n), label, "break")
			}
		}
	case *ast.ContinueStatement:
		// If the continue specifies a label, ensure that it exists.
		if n.Label != nil {
			label := tokens.Name(n.Label.Ident)
			if _, has := a.labels[label]; !has {
				a.b.Diag().Errorf(errors.ErrorUnknownJumpLabel.At(n), label, "continue")
			}
		}
	case *ast.IfStatement:
		// Ensure that the condition is a boolean expression.
		a.checkExprType(n.Condition, types.Bool)
	case *ast.ReturnStatement:
		// Ensure that the return expression is correct (present or missing; and its type).
		fncty := a.b.requireFunctionType(a.fnc)
		if fncty.Return == nil {
			if n.Expression != nil {
				// The function has no return type ("void"), and yet the return had an expression.
				a.b.Diag().Errorf(errors.ErrorUnexpectedReturnExpr.At(*n.Expression))
			}
		} else {
			if n.Expression == nil {
				// The function has a return type, but there was no return expression.
				a.b.Diag().Errorf(errors.ErrorExpectedReturnExpr.At(n))
			} else {
				// Ensure that the returned expression is convertible to the expected return type.
				a.checkExprType(*n.Expression, fncty.Return)
			}
		}
	case *ast.ThrowStatement:
		// TODO: check that the type is an exception.
	case *ast.WhileStatement:
		// Ensure that the loop test is a boolean expression.
		a.checkExprType(n.Test, types.Bool)

	// Expressions
	case *ast.NullLiteral:
		a.b.registerExprType(n, types.Null)
	case *ast.BoolLiteral:
		a.b.registerExprType(n, types.Bool)
	case *ast.NumberLiteral:
		a.b.registerExprType(n, types.Number)
	case *ast.StringLiteral:
		a.b.registerExprType(n, types.String)
	case *ast.ArrayLiteral:
		// If there is a size, ensure it's a number.
		if n.Size != nil {
			a.checkExprType(*n.Size, types.Number)
		}
		// Now mark the resulting expression as an array of the right type.
		if n.ElemType == nil {
			a.b.registerExprType(n, types.AnyArray)
		} else {
			elemType := a.b.bindType(n.ElemType)
			a.b.registerExprType(n, symbols.NewArrayType(elemType))

			// Ensure the elements, if any, are of the right type.
			if n.Elements != nil {
				for _, elem := range *n.Elements {
					a.checkExprType(elem, elemType)
				}
			}
		}
	case *ast.ObjectLiteral:
		// Mark the resulting object literal with the correct type.
		if n.Type == nil {
			a.b.registerExprType(n, types.Any)
		} else {
			a.b.registerExprType(n, a.b.bindType(n.Type))
			// TODO: ensure the properties, if any, actually exist on the target object.
		}
	case *ast.LoadLocationExpression:
		// Create a pointer to the target location.
		// TODO: what to do about readonly variables.
		var sym symbols.Symbol
		if n.Object == nil {
			// If there is no object, we either have a local variable reference or a module property or function.  In
			// the former case, the token will be "simple"; in the latter case, it will be qualified.
			sym = a.b.requireToken(n.Name, n.Name.Tok)
		} else {
			// If there's an object, we are accessing a class member property or function.
			typ := a.b.requireExprType(*n.Object)
			sym = a.b.requireClassMember(n.Name, typ, tokens.ClassMember(n.Name.Tok))
		}

		// TODO: create a pointer type.

		if sym == nil {
			a.b.registerExprType(n, types.Any)
		} else {
			switch s := sym.(type) {
			case ast.Function:
				a.b.registerExprType(n, a.b.requireFunctionType(s))
			case ast.Variable:
				a.b.registerExprType(n, a.b.requireVariableType(s))
			default:
				contract.Failf("Unrecognized load location symbol type: %v", sym.Token())
			}
		}
	case *ast.LoadDynamicExpression:
		a.b.registerExprType(n, types.Any)
	case *ast.NewExpression:
		// TODO: check the arguments.
		if n.Type == nil {
			a.b.registerExprType(n, types.Any)
		} else {
			// TODO: this identifier is expected to be fully bound and hence a full token (not a lexical name).
			a.b.registerExprType(n, a.b.bindType(n.Type))
		}
	case *ast.InvokeFunctionExpression:
		// TODO: ensure the target is a function type.
		// TODO: check the arguments.
		// TODO: the result of this invocation is the return type.
	case *ast.LambdaExpression:
		var params []symbols.Type
		if pparams := n.GetParameters(); pparams != nil {
			for _, param := range *pparams {
				params = append(params, a.b.requireVariableType(param))
			}
		}
		var ret symbols.Type
		if pret := n.GetReturnType(); pret != nil {
			ret = a.b.bindType(pret)
		}
		a.b.registerExprType(n, symbols.NewFunctionType(params, ret))
	case *ast.UnaryOperatorExpression:
		// TODO: check operands.
		// TODO: figure this out; almost certainly a number.
	case *ast.BinaryOperatorExpression:
		// TODO: check operands.
		// TODO: figure this out; almost certainly a number.
	case *ast.CastExpression:
		// TODO: validate that this is legal.
		a.b.registerExprType(n, a.b.bindType(n.Type))
	case *ast.TypeOfExpression:
		// TODO: not sure; a string?
	case *ast.ConditionalExpression:
		// TODO: unify the consequent and alternate types.
	case *ast.SequenceExpression:
		// The type of a sequence expression is just the type of the last expression in the sequence.
		// TODO: check that there's at least one!
		a.b.registerExprType(n, a.b.requireExprType(n.Expressions[len(n.Expressions)-1]))
	}

	// Ensure that all expression types resulted in a type registration.
	expr, isExpr := node.(ast.Expression)
	contract.Assert(!isExpr || a.b.requireExprType(expr) != nil)
}

func (a *astBinder) checkExprType(expr ast.Expression, expect symbols.Type) bool {
	actual := a.b.requireExprType(expr)
	if !types.CanConvert(actual, expect) {
		a.b.Diag().Errorf(errors.ErrorIncorrectExprType.At(expr), expect, actual)
		return false
	}
	return true
}
