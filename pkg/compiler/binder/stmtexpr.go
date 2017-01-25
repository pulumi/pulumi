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
		a.visitBlock(n)
	case *ast.LocalVariable:
		a.visitLocalVariable(n)
	case *ast.LabeledStatement:
		a.visitLabeledStatement(n)
	}

	// Return the current visitor to keep on visitin'.
	return a
}

func (a *astBinder) After(node ast.Node) {
	switch n := node.(type) {
	// Statements
	case *ast.Block:
		a.checkBlock(n)
	case *ast.BreakStatement:
		a.checkBreakStatement(n)
	case *ast.ContinueStatement:
		a.checkContinueStatement(n)
	case *ast.IfStatement:
		a.checkIfStatement(n)
	case *ast.ReturnStatement:
		a.checkReturnStatement(n)
	case *ast.ThrowStatement:
		a.checkThrowStatement(n)
	case *ast.WhileStatement:
		a.checkWhileStatement(n)

	// Expressions
	case *ast.NullLiteral:
		a.b.ctx.RegisterType(n, types.Null) // register a null type.
	case *ast.BoolLiteral:
		a.b.ctx.RegisterType(n, types.Bool) // register a bool type.
	case *ast.NumberLiteral:
		a.b.ctx.RegisterType(n, types.Number) // register as a number type.
	case *ast.StringLiteral:
		a.b.ctx.RegisterType(n, types.String) // register as a string type.
	case *ast.ArrayLiteral:
		a.checkArrayLiteral(n)
	case *ast.ObjectLiteral:
		a.checkObjectLiteral(n)
	case *ast.LoadLocationExpression:
		a.checkLoadLocationExpression(n)
	case *ast.LoadDynamicExpression:
		a.b.ctx.RegisterType(n, types.Any) // register as an any type.
	case *ast.NewExpression:
		a.checkNewExpression(n)
	case *ast.InvokeFunctionExpression:
		a.checkInvokeFunctionExpression(n)
	case *ast.LambdaExpression:
		a.checkLambdaExpression(n)
	case *ast.UnaryOperatorExpression:
		a.checkUnaryOperatorExpression(n)
	case *ast.BinaryOperatorExpression:
		a.checkBinaryOperatorExpression(n)
	case *ast.CastExpression:
		a.checkCastExpression(n)
	case *ast.TypeOfExpression:
		a.checkTypeOfExpression(n)
	case *ast.ConditionalExpression:
		a.checkConditionalExpression(n)
	case *ast.SequenceExpression:
		a.checkSequenceExpression(n)
	}

	// Ensure that all expression types resulted in a type registration.
	expr, isExpr := node.(ast.Expression)
	contract.Assert(!isExpr || a.b.ctx.RequireType(expr) != nil)
}

// Statements

func (a *astBinder) visitBlock(node *ast.Block) {
	// Entering a new block requires a fresh lexical scope.
	a.b.ctx.Scope.Push(false)
}

func (a *astBinder) checkBlock(node *ast.Block) {
	// Exiting a block restores the prior lexical context.
	a.b.ctx.Scope.Pop()
}

func (a *astBinder) checkBreakStatement(node *ast.BreakStatement) {
	// If the break specifies a label, ensure that it exists.
	if node.Label != nil {
		label := tokens.Name(node.Label.Ident)
		if _, has := a.labels[label]; !has {
			a.b.Diag().Errorf(errors.ErrorUnknownJumpLabel.At(node), label, "break")
		}
	}
}

func (a *astBinder) checkContinueStatement(node *ast.ContinueStatement) {
	// If the continue specifies a label, ensure that it exists.
	if node.Label != nil {
		label := tokens.Name(node.Label.Ident)
		if _, has := a.labels[label]; !has {
			a.b.Diag().Errorf(errors.ErrorUnknownJumpLabel.At(node), label, "continue")
		}
	}
}

func (a *astBinder) checkIfStatement(node *ast.IfStatement) {
	// Ensure that the condition is a boolean expression.
	a.checkExprType(node.Condition, types.Bool)
}

func (a *astBinder) visitLocalVariable(node *ast.LocalVariable) {
	// Encountering a new local variable results in registering it; both to the type and symbol table.
	ty := a.b.bindType(node.Type)
	sym := symbols.NewLocalVariableSym(node, ty)
	a.b.ctx.RegisterSymbol(node, ty)
	a.b.ctx.Scope.TryRegister(node, sym) // TODO: figure out whether to keep this.
}

func (a *astBinder) visitLabeledStatement(node *ast.LabeledStatement) {
	// Ensure this label doesn't already exist and then register it.
	label := tokens.Name(node.Label.Ident)
	if other, has := a.labels[label]; has {
		a.b.Diag().Errorf(errors.ErrorDuplicateLabel.At(node), label, other)
	}
	a.labels[label] = node
}

func (a *astBinder) checkReturnStatement(node *ast.ReturnStatement) {
	// Ensure that the return expression is correct (present or missing; and its type).
	fncty := a.b.ctx.RequireFunction(a.fnc).Type()
	if fncty.Return == nil {
		if node.Expression != nil {
			// The function has no return type ("void"), and yet the return had an expression.
			a.b.Diag().Errorf(errors.ErrorUnexpectedReturnExpr.At(*node.Expression))
		}
	} else {
		if node.Expression == nil {
			// The function has a return type, but there was no return expression.
			a.b.Diag().Errorf(errors.ErrorExpectedReturnExpr.At(node))
		} else {
			// Ensure that the returned expression is convertible to the expected return type.
			a.checkExprType(*node.Expression, fncty.Return)
		}
	}
}

func (a *astBinder) checkThrowStatement(node *ast.ThrowStatement) {
	// TODO: ensure the expression is a throwable expression.
}

func (a *astBinder) checkWhileStatement(node *ast.WhileStatement) {
	// Ensure that the loop test is a boolean expression.
	a.checkExprType(node.Test, types.Bool)
}

// Expressions

func (a *astBinder) checkExprType(expr ast.Expression, expect symbols.Type) bool {
	actual := a.b.ctx.RequireType(expr)
	if !types.CanConvert(actual, expect) {
		a.b.Diag().Errorf(errors.ErrorIncorrectExprType.At(expr), expect, actual)
		return false
	}
	return true
}

func (a *astBinder) checkArrayLiteral(node *ast.ArrayLiteral) {
	// If there is a size, ensure it's a number.
	if node.Size != nil {
		a.checkExprType(*node.Size, types.Number)
	}
	// Now mark the resulting expression as an array of the right type.
	if node.ElemType == nil {
		a.b.ctx.RegisterType(node, types.AnyArray)
	} else {
		elemType := a.b.bindType(node.ElemType)
		a.b.ctx.RegisterType(node, symbols.NewArrayType(elemType))

		// Ensure the elements, if any, are of the right type.
		if node.Elements != nil {
			for _, elem := range *node.Elements {
				a.checkExprType(elem, elemType)
			}
		}
	}
}

func (a *astBinder) checkObjectLiteral(node *ast.ObjectLiteral) {
	// Mark the resulting object literal with the correct type.
	if node.Type == nil {
		a.b.ctx.RegisterType(node, types.Any)
	} else {
		ty := a.b.bindType(node.Type)
		a.b.ctx.RegisterType(node, ty)

		// Only permit object literals for records and interfaces.  Classes have constructors.
		if !ty.Record() && !ty.Interface() {
			a.b.Diag().Errorf(errors.ErrorIllegalObjectLiteralType.At(node.Type), ty)
		} else {
			// Ensure that all required properties have been supplied, and that they are of the right type.
			props := make(map[tokens.ClassMemberName]bool)
			if node.Properties != nil {
				for _, init := range *node.Properties {
					sym := a.b.requireClassMember(init.Property, ty, init.Property.Tok)
					if sym != nil {
						switch s := sym.(type) {
						case *symbols.ClassProperty, *symbols.ClassMethod:
							a.checkExprType(init.Value, s.Type())
						default:
							contract.Failf("Unrecognized class member symbol: %v", sym)
						}
						props[init.Property.Tok.Name()] = true // record that we've seen this one.
					}
				}
			}

			// Issue an error about any missing required properties.
			for name, member := range ty.TypeMembers() {
				if _, has := props[name]; !has {
					if !member.Optional() && member.Default() == nil {
						a.b.Diag().Errorf(errors.ErrorMissingRequiredProperty.At(node), name)
					}
				}
			}
		}
	}
}

func (a *astBinder) checkLoadLocationExpression(node *ast.LoadLocationExpression) {
	// Bind the token to a location.
	// TODO: what to do about readonly variables.
	var sym symbols.Symbol
	if node.Object == nil {
		// If there is no object, we either have a local variable reference or a module property or function.  In
		// the former case, the token will be "simple"; in the latter case, it will be qualified.
		sym = a.b.requireToken(node.Name, node.Name.Tok)
	} else {
		// If there's an object, we are accessing a class member property or function.
		typ := a.b.ctx.RequireType(*node.Object)
		sym = a.b.requireClassMember(node.Name, typ, tokens.ClassMember(node.Name.Tok))
	}

	// Produce a type of the right kind from the target location.
	var ty symbols.Type
	if sym == nil {
		ty = types.Any
	} else {
		switch s := sym.(type) {
		case ast.Function:
			ty = a.b.ctx.RequireFunction(s).Type()
		case ast.Variable:
			ty = a.b.ctx.RequireVariable(s).Type()
		default:
			contract.Failf("Unrecognized load location symbol type: %v", sym.Token())
		}
	}

	// Register a pointer type so that this expression is a valid l-expr.
	a.b.ctx.RegisterType(node, symbols.NewPointerType(ty))
}

func (a *astBinder) checkNewExpression(node *ast.NewExpression) {
	// TODO: check the arguments.
	var ty symbols.Type
	if node.Type == nil {
		ty = types.Any
	} else {
		ty = a.b.bindType(node.Type)
		if class, isclass := ty.(*symbols.Class); isclass {
			// Ensure we're not creating an abstract class.
			if class.Abstract() {
				a.b.Diag().Errorf(errors.ErrorCannotNewAbstractClass.At(node.Type), class)
			}
		}
	}

	a.b.ctx.RegisterType(node, ty)
}

func (a *astBinder) checkInvokeFunctionExpression(node *ast.InvokeFunctionExpression) {
	// TODO: ensure the target is a function type.
	// TODO: check the arguments.
	// TODO: the result of this invocation is the return type.
}

func (a *astBinder) checkLambdaExpression(node *ast.LambdaExpression) {
	var params []symbols.Type
	if pparams := node.GetParameters(); pparams != nil {
		for _, param := range *pparams {
			params = append(params, a.b.ctx.RequireVariable(param).Type())
		}
	}
	var ret symbols.Type
	if pret := node.GetReturnType(); pret != nil {
		ret = a.b.bindType(pret)
	}
	a.b.ctx.RegisterType(node, symbols.NewFunctionType(params, ret))
}

func (a *astBinder) checkUnaryOperatorExpression(node *ast.UnaryOperatorExpression) {
	// TODO: check operands.
	// TODO: figure this out; almost certainly a number.
}

func (a *astBinder) checkBinaryOperatorExpression(node *ast.BinaryOperatorExpression) {
	// TODO: check operands.
	// TODO: figure this out; almost certainly a number.
}

func (a *astBinder) checkCastExpression(node *ast.CastExpression) {
	// TODO: validate that this is legal.
	a.b.ctx.RegisterType(node, a.b.bindType(node.Type))
}

func (a *astBinder) checkTypeOfExpression(node *ast.TypeOfExpression) {
	// TODO: not sure; a string?
}

func (a *astBinder) checkConditionalExpression(node *ast.ConditionalExpression) {
	// TODO: unify the consequent and alternate types.
}

func (a *astBinder) checkSequenceExpression(node *ast.SequenceExpression) {
	// The type of a sequence expression is just the type of the last expression in the sequence.
	// TODO: check that there's at least one!
	a.b.ctx.RegisterType(node, a.b.ctx.RequireType(node.Expressions[len(node.Expressions)-1]))
}
